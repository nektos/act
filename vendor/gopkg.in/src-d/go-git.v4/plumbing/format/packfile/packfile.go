package packfile

import (
	"bytes"
	"io"
	"os"

	billy "gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/format/idxfile"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

var (
	// ErrInvalidObject is returned by Decode when an invalid object is
	// found in the packfile.
	ErrInvalidObject = NewError("invalid git object")
	// ErrZLib is returned by Decode when there was an error unzipping
	// the packfile contents.
	ErrZLib = NewError("zlib reading error")
)

// Packfile allows retrieving information from inside a packfile.
type Packfile struct {
	idxfile.Index
	fs             billy.Filesystem
	file           billy.File
	s              *Scanner
	deltaBaseCache cache.Object
	offsetToType   map[int64]plumbing.ObjectType
}

// NewPackfileWithCache creates a new Packfile with the given object cache.
// If the filesystem is provided, the packfile will return FSObjects, otherwise
// it will return MemoryObjects.
func NewPackfileWithCache(
	index idxfile.Index,
	fs billy.Filesystem,
	file billy.File,
	cache cache.Object,
) *Packfile {
	s := NewScanner(file)
	return &Packfile{
		index,
		fs,
		file,
		s,
		cache,
		make(map[int64]plumbing.ObjectType),
	}
}

// NewPackfile returns a packfile representation for the given packfile file
// and packfile idx.
// If the filesystem is provided, the packfile will return FSObjects, otherwise
// it will return MemoryObjects.
func NewPackfile(index idxfile.Index, fs billy.Filesystem, file billy.File) *Packfile {
	return NewPackfileWithCache(index, fs, file, cache.NewObjectLRUDefault())
}

// Get retrieves the encoded object in the packfile with the given hash.
func (p *Packfile) Get(h plumbing.Hash) (plumbing.EncodedObject, error) {
	offset, err := p.FindOffset(h)
	if err != nil {
		return nil, err
	}

	return p.GetByOffset(offset)
}

// GetByOffset retrieves the encoded object from the packfile with the given
// offset.
func (p *Packfile) GetByOffset(o int64) (plumbing.EncodedObject, error) {
	hash, err := p.FindHash(o)
	if err == nil {
		if obj, ok := p.deltaBaseCache.Get(hash); ok {
			return obj, nil
		}
	}

	if _, err := p.s.SeekFromStart(o); err != nil {
		if err == io.EOF || isInvalid(err) {
			return nil, plumbing.ErrObjectNotFound
		}

		return nil, err
	}

	return p.nextObject()
}

// GetSizeByOffset retrieves the size of the encoded object from the
// packfile with the given offset.
func (p *Packfile) GetSizeByOffset(o int64) (size int64, err error) {
	if _, err := p.s.SeekFromStart(o); err != nil {
		if err == io.EOF || isInvalid(err) {
			return 0, plumbing.ErrObjectNotFound
		}

		return 0, err
	}

	h, err := p.nextObjectHeader()
	if err != nil {
		return 0, err
	}
	return h.Length, nil
}

func (p *Packfile) nextObjectHeader() (*ObjectHeader, error) {
	h, err := p.s.NextObjectHeader()
	p.s.pendingObject = nil
	return h, err
}

func (p *Packfile) getObjectSize(h *ObjectHeader) (int64, error) {
	switch h.Type {
	case plumbing.CommitObject, plumbing.TreeObject, plumbing.BlobObject, plumbing.TagObject:
		return h.Length, nil
	case plumbing.REFDeltaObject, plumbing.OFSDeltaObject:
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufPool.Put(buf)

		if _, _, err := p.s.NextObject(buf); err != nil {
			return 0, err
		}

		delta := buf.Bytes()
		_, delta = decodeLEB128(delta) // skip src size
		sz, _ := decodeLEB128(delta)
		return int64(sz), nil
	default:
		return 0, ErrInvalidObject.AddDetails("type %q", h.Type)
	}
}

func (p *Packfile) getObjectType(h *ObjectHeader) (typ plumbing.ObjectType, err error) {
	switch h.Type {
	case plumbing.CommitObject, plumbing.TreeObject, plumbing.BlobObject, plumbing.TagObject:
		return h.Type, nil
	case plumbing.REFDeltaObject, plumbing.OFSDeltaObject:
		var offset int64
		if h.Type == plumbing.REFDeltaObject {
			offset, err = p.FindOffset(h.Reference)
			if err != nil {
				return
			}
		} else {
			offset = h.OffsetReference
		}

		if baseType, ok := p.offsetToType[offset]; ok {
			typ = baseType
		} else {
			if _, err = p.s.SeekFromStart(offset); err != nil {
				return
			}

			h, err = p.nextObjectHeader()
			if err != nil {
				return
			}

			typ, err = p.getObjectType(h)
			if err != nil {
				return
			}
		}
	default:
		err = ErrInvalidObject.AddDetails("type %q", h.Type)
	}

	return
}

func (p *Packfile) nextObject() (plumbing.EncodedObject, error) {
	h, err := p.nextObjectHeader()
	if err != nil {
		if err == io.EOF || isInvalid(err) {
			return nil, plumbing.ErrObjectNotFound
		}
		return nil, err
	}

	// If we have no filesystem, we will return a MemoryObject instead
	// of an FSObject.
	if p.fs == nil {
		return p.getNextObject(h)
	}

	hash, err := p.FindHash(h.Offset)
	if err != nil {
		return nil, err
	}

	size, err := p.getObjectSize(h)
	if err != nil {
		return nil, err
	}

	typ, err := p.getObjectType(h)
	if err != nil {
		return nil, err
	}

	p.offsetToType[h.Offset] = typ

	return NewFSObject(
		hash,
		typ,
		h.Offset,
		size,
		p.Index,
		p.fs,
		p.file.Name(),
		p.deltaBaseCache,
	), nil
}

func (p *Packfile) getObjectContent(offset int64) (io.ReadCloser, error) {
	ref, err := p.FindHash(offset)
	if err == nil {
		obj, ok := p.cacheGet(ref)
		if ok {
			reader, err := obj.Reader()
			if err != nil {
				return nil, err
			}

			return reader, nil
		}
	}

	if _, err := p.s.SeekFromStart(offset); err != nil {
		return nil, err
	}

	h, err := p.nextObjectHeader()
	if err != nil {
		return nil, err
	}

	obj, err := p.getNextObject(h)
	if err != nil {
		return nil, err
	}

	return obj.Reader()
}

func (p *Packfile) getNextObject(h *ObjectHeader) (plumbing.EncodedObject, error) {
	var obj = new(plumbing.MemoryObject)
	obj.SetSize(h.Length)
	obj.SetType(h.Type)

	var err error
	switch h.Type {
	case plumbing.CommitObject, plumbing.TreeObject, plumbing.BlobObject, plumbing.TagObject:
		err = p.fillRegularObjectContent(obj)
	case plumbing.REFDeltaObject:
		err = p.fillREFDeltaObjectContent(obj, h.Reference)
	case plumbing.OFSDeltaObject:
		err = p.fillOFSDeltaObjectContent(obj, h.OffsetReference)
	default:
		err = ErrInvalidObject.AddDetails("type %q", h.Type)
	}

	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (p *Packfile) fillRegularObjectContent(obj plumbing.EncodedObject) error {
	w, err := obj.Writer()
	if err != nil {
		return err
	}

	_, _, err = p.s.NextObject(w)
	p.cachePut(obj)

	return err
}

func (p *Packfile) fillREFDeltaObjectContent(obj plumbing.EncodedObject, ref plumbing.Hash) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	_, _, err := p.s.NextObject(buf)
	if err != nil {
		return err
	}

	base, ok := p.cacheGet(ref)
	if !ok {
		base, err = p.Get(ref)
		if err != nil {
			return err
		}
	}

	obj.SetType(base.Type())
	err = ApplyDelta(obj, base, buf.Bytes())
	p.cachePut(obj)
	bufPool.Put(buf)

	return err
}

func (p *Packfile) fillOFSDeltaObjectContent(obj plumbing.EncodedObject, offset int64) error {
	buf := bytes.NewBuffer(nil)
	_, _, err := p.s.NextObject(buf)
	if err != nil {
		return err
	}

	var base plumbing.EncodedObject
	var ok bool
	hash, err := p.FindHash(offset)
	if err == nil {
		base, ok = p.cacheGet(hash)
	}

	if !ok {
		base, err = p.GetByOffset(offset)
		if err != nil {
			return err
		}

		p.cachePut(base)
	}

	obj.SetType(base.Type())
	err = ApplyDelta(obj, base, buf.Bytes())
	p.cachePut(obj)

	return err
}

func (p *Packfile) cacheGet(h plumbing.Hash) (plumbing.EncodedObject, bool) {
	if p.deltaBaseCache == nil {
		return nil, false
	}

	return p.deltaBaseCache.Get(h)
}

func (p *Packfile) cachePut(obj plumbing.EncodedObject) {
	if p.deltaBaseCache == nil {
		return
	}

	p.deltaBaseCache.Put(obj)
}

// GetAll returns an iterator with all encoded objects in the packfile.
// The iterator returned is not thread-safe, it should be used in the same
// thread as the Packfile instance.
func (p *Packfile) GetAll() (storer.EncodedObjectIter, error) {
	return p.GetByType(plumbing.AnyObject)
}

// GetByType returns all the objects of the given type.
func (p *Packfile) GetByType(typ plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	switch typ {
	case plumbing.AnyObject,
		plumbing.BlobObject,
		plumbing.TreeObject,
		plumbing.CommitObject,
		plumbing.TagObject:
		entries, err := p.EntriesByOffset()
		if err != nil {
			return nil, err
		}

		return &objectIter{
			// Easiest way to provide an object decoder is just to pass a Packfile
			// instance. To not mess with the seeks, it's a new instance with a
			// different scanner but the same cache and offset to hash map for
			// reusing as much cache as possible.
			p:    p,
			iter: entries,
			typ:  typ,
		}, nil
	default:
		return nil, plumbing.ErrInvalidType
	}
}

// ID returns the ID of the packfile, which is the checksum at the end of it.
func (p *Packfile) ID() (plumbing.Hash, error) {
	prev, err := p.file.Seek(-20, io.SeekEnd)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	var hash plumbing.Hash
	if _, err := io.ReadFull(p.file, hash[:]); err != nil {
		return plumbing.ZeroHash, err
	}

	if _, err := p.file.Seek(prev, io.SeekStart); err != nil {
		return plumbing.ZeroHash, err
	}

	return hash, nil
}

// Close the packfile and its resources.
func (p *Packfile) Close() error {
	closer, ok := p.file.(io.Closer)
	if !ok {
		return nil
	}

	return closer.Close()
}

type objectIter struct {
	p    *Packfile
	typ  plumbing.ObjectType
	iter idxfile.EntryIter
}

func (i *objectIter) Next() (plumbing.EncodedObject, error) {
	for {
		e, err := i.iter.Next()
		if err != nil {
			return nil, err
		}

		obj, err := i.p.GetByOffset(int64(e.Offset))
		if err != nil {
			return nil, err
		}

		if i.typ == plumbing.AnyObject || obj.Type() == i.typ {
			return obj, nil
		}
	}
}

func (i *objectIter) ForEach(f func(plumbing.EncodedObject) error) error {
	for {
		o, err := i.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if err := f(o); err != nil {
			return err
		}
	}
}

func (i *objectIter) Close() {
	i.iter.Close()
}

// isInvalid checks whether an error is an os.PathError with an os.ErrInvalid
// error inside. It also checks for the windows error, which is different from
// os.ErrInvalid.
func isInvalid(err error) bool {
	pe, ok := err.(*os.PathError)
	if !ok {
		return false
	}

	errstr := pe.Err.Error()
	return errstr == errInvalidUnix || errstr == errInvalidWindows
}

// errInvalidWindows is the Windows equivalent to os.ErrInvalid
const errInvalidWindows = "The parameter is incorrect."

var errInvalidUnix = os.ErrInvalid.Error()
