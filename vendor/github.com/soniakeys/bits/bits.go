// Copyright 2017 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

// Bits implements methods on a bit array type.
//
// The Bits type holds a fixed size array of bits, numbered consecutively
// from zero.  Some set-like operations are possible, but the API is more
// array-like or register-like.
package bits

import (
	"fmt"
	mb "math/bits"
)

// Bits holds a fixed number of bits.
//
// Bit number 0 is stored in the LSB, or bit 0, of the word indexed at 0.
//
// When Num is not a multiple of 64, the last element of Bits will hold some
// bits beyond Num.  These bits are undefined.  They are not required to be
// zero but do not have any meaning.  Bits methods are not required to leave
// them undisturbed.
type Bits struct {
	Num  int // number of bits
	Bits []uint64
}

// New constructs a Bits value with the given number of bits.
//
// It panics if num is negative.
func New(num int) Bits {
	if num < 0 {
		panic("negative number of bits")
	}
	return Bits{num, make([]uint64, (num+63)>>6)}
}

// NewGivens constructs a Bits value with the given bits nums set to 1.
//
// The number of bits will be just enough to hold the largest bit value
// listed.  That is, the number of bits will be the max bit number plus one.
//
// It panics if any bit number is negative.
func NewGivens(nums ...int) Bits {
	max := -1
	for _, p := range nums {
		if p > max {
			max = p
		}
	}
	b := New(max + 1)
	for _, p := range nums {
		b.SetBit(p, 1)
	}
	return b
}

// AllOnes returns true if all Num bits are 1.
func (b Bits) AllOnes() bool {
	last := len(b.Bits) - 1
	for _, w := range b.Bits[:last] {
		if w != ^uint64(0) {
			return false
		}
	}
	return ^b.Bits[last]<<uint(64*len(b.Bits)-b.Num) == 0
}

// AllZeros returns true if all Num bits are 0.
func (b Bits) AllZeros() bool {
	last := len(b.Bits) - 1
	for _, w := range b.Bits[:last] {
		if w != 0 {
			return false
		}
	}
	return b.Bits[last]<<uint(64*len(b.Bits)-b.Num) == 0
}

// And sets z = x & y.
//
// It panics if x and y do not have the same Num.
func (z *Bits) And(x, y Bits) {
	if x.Num != y.Num {
		panic("arguments have different number of bits")
	}
	if z.Num != x.Num {
		*z = New(x.Num)
	}
	for i, w := range y.Bits {
		z.Bits[i] = x.Bits[i] & w
	}
}

// AndNot sets z = x &^ y.
//
// It panics if x and y do not have the same Num.
func (z *Bits) AndNot(x, y Bits) {
	if x.Num != y.Num {
		panic("arguments have different number of bits")
	}
	if z.Num != x.Num {
		*z = New(x.Num)
	}
	for i, w := range y.Bits {
		z.Bits[i] = x.Bits[i] &^ w
	}
}

// Bit returns the value of the n'th bit of receiver b.
func (b Bits) Bit(n int) int {
	if n < 0 || n >= b.Num {
		panic("bit number out of range")
	}
	return int(b.Bits[n>>6] >> uint(n&63) & 1)
}

// ClearAll sets all bits to 0.
func (b Bits) ClearAll() {
	for i := range b.Bits {
		b.Bits[i] = 0
	}
}

// ClearBits sets the given bits to 0 in receiver b.
//
// Other bits of b are left unchanged.
//
// It panics if any bit number is out of range.
// That is, negative or >= the number of bits.
func (b Bits) ClearBits(nums ...int) {
	for _, p := range nums {
		b.SetBit(p, 0)
	}
}

// Equal returns true if all Num bits of a and b are equal.
//
// It panics if a and b have different Num.
func (a Bits) Equal(b Bits) bool {
	if a.Num != b.Num {
		panic("receiver and argument have different number of bits")
	}
	if a.Num == 0 {
		return true
	}
	last := len(a.Bits) - 1
	for i, w := range a.Bits[:last] {
		if w != b.Bits[i] {
			return false
		}
	}
	return (a.Bits[last]^b.Bits[last])<<uint(len(a.Bits)*64-a.Num) == 0
}

// IterateOnes calls visitor function v for each bit with a value of 1, in order
// from lowest bit to highest bit.
//
// Iteration continues to the highest bit as long as v returns true.
// It stops if v returns false.
//
// IterateOnes returns true normally.  It returns false if v returns false.
//
// IterateOnes may not be sensitive to changes if bits are changed during
// iteration, by the vistor function for example.
// See OneFrom for an iteration method sensitive to changes during iteration.
func (b Bits) IterateOnes(v func(int) bool) bool {
	for x, w := range b.Bits {
		if w != 0 {
			t := mb.TrailingZeros64(w)
			i := t // index in w of next 1 bit
			for {
				n := x<<6 | i
				if n >= b.Num {
					return true
				}
				if !v(x<<6 | i) {
					return false
				}
				w >>= uint(t + 1)
				if w == 0 {
					break
				}
				t = mb.TrailingZeros64(w)
				i += 1 + t
			}
		}
	}
	return true
}

// IterateZeros calls visitor function v for each bit with a value of 0,
// in order from lowest bit to highest bit.
//
// Iteration continues to the highest bit as long as v returns true.
// It stops if v returns false.
//
// IterateZeros returns true normally.  It returns false if v returns false.
//
// IterateZeros may not be sensitive to changes if bits are changed during
// iteration, by the vistor function for example.
// See ZeroFrom for an iteration method sensitive to changes during iteration.
func (b Bits) IterateZeros(v func(int) bool) bool {
	for x, w := range b.Bits {
		w = ^w
		if w != 0 {
			t := mb.TrailingZeros64(w)
			i := t // index in w of next 1 bit
			for {
				n := x<<6 | i
				if n >= b.Num {
					return true
				}
				if !v(x<<6 | i) {
					return false
				}
				w >>= uint(t + 1)
				if w == 0 {
					break
				}
				t = mb.TrailingZeros64(w)
				i += 1 + t
			}
		}
	}
	return true
}

// Not sets receiver z to the complement of b.
func (z *Bits) Not(b Bits) {
	if z.Num != b.Num {
		*z = New(b.Num)
	}
	for i, w := range b.Bits {
		z.Bits[i] = ^w
	}
}

// OneFrom returns the number of the first 1 bit at or after (from) bit num.
//
// It returns -1 if there is no one bit at or after num.
//
// This provides one way to iterate over one bits.
// To iterate over the one bits, call OneFrom with n = 0 to get the the first
// one bit, then call with the result + 1 to get successive one bits.
// Unlike the Iterate method, this technique is stateless and so allows
// bits to be changed between successive calls.
//
// There is no panic for calling OneFrom with an argument >= b.Num.
// In this case OneFrom simply returns -1.
//
// See also Iterate.
func (b Bits) OneFrom(num int) int {
	if num >= b.Num {
		return -1
	}
	x := num >> 6
	// test for 1 in this word at or after n
	if wx := b.Bits[x] >> uint(num&63); wx != 0 {
		num += mb.TrailingZeros64(wx)
		if num >= b.Num {
			return -1
		}
		return num
	}
	x++
	for y, wy := range b.Bits[x:] {
		if wy != 0 {
			num = (x+y)<<6 | mb.TrailingZeros64(wy)
			if num >= b.Num {
				return -1
			}
			return num
		}
	}
	return -1
}

// Or sets z = x | y.
//
// It panics if x and y do not have the same Num.
func (z *Bits) Or(x, y Bits) {
	if x.Num != y.Num {
		panic("arguments have different number of bits")
	}
	if z.Num != x.Num {
		*z = New(x.Num)
	}
	for i, w := range y.Bits {
		z.Bits[i] = x.Bits[i] | w
	}
}

// OnesCount returns the number of 1 bits.
func (b Bits) OnesCount() (c int) {
	if b.Num == 0 {
		return 0
	}
	last := len(b.Bits) - 1
	for _, w := range b.Bits[:last] {
		c += mb.OnesCount64(w)
	}
	c += mb.OnesCount64(b.Bits[last] << uint(len(b.Bits)*64-b.Num))
	return
}

// Set sets the bits of z to the bits of x.
func (z *Bits) Set(b Bits) {
	if z.Num != b.Num {
		*z = New(b.Num)
	}
	copy(z.Bits, b.Bits)
}

// SetAll sets z to have all 1 bits.
func (b Bits) SetAll() {
	for i := range b.Bits {
		b.Bits[i] = ^uint64(0)
	}
}

// SetBit sets the n'th bit to x, where x is a 0 or 1.
//
// It panics if n is out of range
func (b Bits) SetBit(n, x int) {
	if n < 0 || n >= b.Num {
		panic("bit number out of range")
	}
	if x == 0 {
		b.Bits[n>>6] &^= 1 << uint(n&63)
	} else {
		b.Bits[n>>6] |= 1 << uint(n&63)
	}
}

// SetBits sets the given bits to 1 in receiver b.
//
// Other bits of b are left unchanged.
//
// It panics if any bit number is out of range, negative or >= the number
// of bits.
func (b Bits) SetBits(nums ...int) {
	for _, p := range nums {
		b.SetBit(p, 1)
	}
}

// Single returns true if b has exactly one 1 bit.
func (b Bits) Single() bool {
	// like OnesCount, but stop as soon as two are found
	if b.Num == 0 {
		return false
	}
	c := 0
	last := len(b.Bits) - 1
	for _, w := range b.Bits[:last] {
		c += mb.OnesCount64(w)
		if c > 1 {
			return false
		}
	}
	c += mb.OnesCount64(b.Bits[last] << uint(len(b.Bits)*64-b.Num))
	return c == 1
}

// Slice returns a slice with the bit numbers of each 1 bit.
func (b Bits) Slice() (s []int) {
	for x, w := range b.Bits {
		if w == 0 {
			continue
		}
		t := mb.TrailingZeros64(w)
		i := t // index in w of next 1 bit
		for {
			n := x<<6 | i
			if n >= b.Num {
				break
			}
			s = append(s, n)
			w >>= uint(t + 1)
			if w == 0 {
				break
			}
			t = mb.TrailingZeros64(w)
			i += 1 + t
		}
	}
	return
}

// String returns a readable representation.
//
// The returned string is big-endian, with the highest number bit first.
//
// If Num is 0, an empty string is returned.
func (b Bits) String() (s string) {
	if b.Num == 0 {
		return ""
	}
	last := len(b.Bits) - 1
	for _, w := range b.Bits[:last] {
		s = fmt.Sprintf("%064b", w) + s
	}
	lb := b.Num - 64*last
	return fmt.Sprintf("%0*b", lb,
		b.Bits[last]&(^uint64(0)>>uint(64-lb))) + s
}

// Xor sets z = x ^ y.
func (z *Bits) Xor(x, y Bits) {
	if x.Num != y.Num {
		panic("arguments have different number of bits")
	}
	if z.Num != x.Num {
		*z = New(x.Num)
	}
	for i, w := range y.Bits {
		z.Bits[i] = x.Bits[i] ^ w
	}
}

// ZeroFrom returns the number of the first 0 bit at or after (from) bit num.
//
// It returns -1 if there is no zero bit at or after num.
//
// This provides one way to iterate over zero bits.
// To iterate over the zero bits, call ZeroFrom with n = 0 to get the the first
// zero bit, then call with the result + 1 to get successive zero bits.
// Unlike the IterateZeros method, this technique is stateless and so allows
// bits to be changed between successive calls.
//
// There is no panic for calling ZeroFrom with an argument >= b.Num.
// In this case ZeroFrom simply returns -1.
//
// See also IterateZeros.
func (b Bits) ZeroFrom(num int) int {
	// code much like OneFrom except words are negated before testing
	if num >= b.Num {
		return -1
	}
	x := num >> 6
	// negate word to test for 0 at or after n
	if wx := ^b.Bits[x] >> uint(num&63); wx != 0 {
		num += mb.TrailingZeros64(wx)
		if num >= b.Num {
			return -1
		}
		return num
	}
	x++
	for y, wy := range b.Bits[x:] {
		wy = ^wy
		if wy != 0 {
			num = (x+y)<<6 | mb.TrailingZeros64(wy)
			if num >= b.Num {
				return -1
			}
			return num
		}
	}
	return -1
}
