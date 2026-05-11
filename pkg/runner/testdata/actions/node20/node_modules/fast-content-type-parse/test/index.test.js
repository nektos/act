'use strict'

const { test } = require('node:test')
const { parse, safeParse } = require('..')

const invalidTypes = [
  ' ',
  'null',
  'undefined',
  '/',
  'text / plain',
  'text/;plain',
  'text/"plain"',
  'text/p£ain',
  'text/(plain)',
  'text/@plain',
  'text/plain,wrong'
]

test('parse', async function (t) {
  t.plan(13 + invalidTypes.length)
  await t.test('should parse basic type', function (t) {
    t.plan(1)
    const type = parse('text/html')
    t.assert.deepStrictEqual(type.type, 'text/html')
  })

  await t.test('should parse with suffix', function (t) {
    t.plan(1)
    const type = parse('image/svg+xml')
    t.assert.deepStrictEqual(type.type, 'image/svg+xml')
  })

  await t.test('should parse basic type with surrounding OWS', function (t) {
    t.plan(1)
    const type = parse(' text/html ')
    t.assert.deepStrictEqual(type.type, 'text/html')
  })

  await t.test('should parse parameters', function (t) {
    t.plan(2)
    const type = parse('text/html; charset=utf-8; foo=bar')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'utf-8',
      foo: 'bar'
    })
  })

  await t.test('should parse parameters with extra LWS', function (t) {
    t.plan(2)
    const type = parse('text/html ; charset=utf-8 ; foo=bar')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'utf-8',
      foo: 'bar'
    })
  })

  await t.test('should lower-case type', function (t) {
    t.plan(1)
    const type = parse('IMAGE/SVG+XML')
    t.assert.deepStrictEqual(type.type, 'image/svg+xml')
  })

  await t.test('should lower-case parameter names', function (t) {
    t.plan(2)
    const type = parse('text/html; Charset=UTF-8')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'UTF-8'
    })
  })

  await t.test('should unquote parameter values', function (t) {
    t.plan(2)
    const type = parse('text/html; charset="UTF-8"')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'UTF-8'
    })
  })

  await t.test('should unquote parameter values with escapes', function (t) {
    t.plan(2)
    const type = parse('text/html; charset="UT\\F-\\\\\\"8\\""')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'UTF-\\"8"'
    })
  })

  await t.test('should handle balanced quotes', function (t) {
    t.plan(2)
    const type = parse('text/html; param="charset=\\"utf-8\\"; foo=bar"; bar=foo')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      param: 'charset="utf-8"; foo=bar',
      bar: 'foo'
    })
  })

  invalidTypes.forEach(async function (type) {
    await t.test('should throw on invalid media type ' + type, function (t) {
      t.plan(1)
      t.assert.throws(parse.bind(null, type), new TypeError('invalid media type'))
    })
  })

  await t.test('should throw on invalid parameter format', function (t) {
    t.plan(3)
    t.assert.throws(parse.bind(null, 'text/plain; foo="bar'), new TypeError('invalid parameter format'))
    t.assert.throws(parse.bind(null, 'text/plain; profile=http://localhost; foo=bar'), new TypeError('invalid parameter format'))
    t.assert.throws(parse.bind(null, 'text/plain; profile=http://localhost'), new TypeError('invalid parameter format'))
  })

  await t.test('should require argument', function (t) {
    t.plan(1)
    // @ts-expect-error should reject non-strings
    t.assert.throws(parse.bind(null), new TypeError('argument header is required and must be a string'))
  })

  await t.test('should reject non-strings', function (t) {
    t.plan(1)
    // @ts-expect-error should reject non-strings
    t.assert.throws(parse.bind(null, 7), new TypeError('argument header is required and must be a string'))
  })
})

test('safeParse', async function (t) {
  t.plan(13 + invalidTypes.length)
  await t.test('should safeParse basic type', function (t) {
    t.plan(1)
    const type = safeParse('text/html')
    t.assert.deepStrictEqual(type.type, 'text/html')
  })

  await t.test('should safeParse with suffix', function (t) {
    t.plan(1)
    const type = safeParse('image/svg+xml')
    t.assert.deepStrictEqual(type.type, 'image/svg+xml')
  })

  await t.test('should safeParse basic type with surrounding OWS', function (t) {
    t.plan(1)
    const type = safeParse(' text/html ')
    t.assert.deepStrictEqual(type.type, 'text/html')
  })

  await t.test('should safeParse parameters', function (t) {
    t.plan(2)
    const type = safeParse('text/html; charset=utf-8; foo=bar')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'utf-8',
      foo: 'bar'
    })
  })

  await t.test('should safeParse parameters with extra LWS', function (t) {
    t.plan(2)
    const type = safeParse('text/html ; charset=utf-8 ; foo=bar')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'utf-8',
      foo: 'bar'
    })
  })

  await t.test('should lower-case type', function (t) {
    t.plan(1)
    const type = safeParse('IMAGE/SVG+XML')
    t.assert.deepStrictEqual(type.type, 'image/svg+xml')
  })

  await t.test('should lower-case parameter names', function (t) {
    t.plan(2)
    const type = safeParse('text/html; Charset=UTF-8')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'UTF-8'
    })
  })

  await t.test('should unquote parameter values', function (t) {
    t.plan(2)
    const type = safeParse('text/html; charset="UTF-8"')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'UTF-8'
    })
  })

  await t.test('should unquote parameter values with escapes', function (t) {
    t.plan(2)
    const type = safeParse('text/html; charset="UT\\F-\\\\\\"8\\""')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      charset: 'UTF-\\"8"'
    })
  })

  await t.test('should handle balanced quotes', function (t) {
    t.plan(2)
    const type = safeParse('text/html; param="charset=\\"utf-8\\"; foo=bar"; bar=foo')
    t.assert.deepStrictEqual(type.type, 'text/html')
    t.assert.deepEqual(type.parameters, {
      param: 'charset="utf-8"; foo=bar',
      bar: 'foo'
    })
  })

  invalidTypes.forEach(async function (type) {
    await t.test('should return dummyContentType on invalid media type ' + type, function (t) {
      t.plan(2)
      t.assert.deepStrictEqual(safeParse(type).type, '')
      t.assert.deepStrictEqual(Object.keys(safeParse(type).parameters).length, 0)
    })
  })

  await t.test('should return dummyContentType on invalid parameter format', function (t) {
    t.plan(6)
    t.assert.deepStrictEqual(safeParse('text/plain; foo="bar').type, '')
    t.assert.deepStrictEqual(Object.keys(safeParse('text/plain; foo="bar').parameters).length, 0)

    t.assert.deepStrictEqual(safeParse('text/plain; profile=http://localhost; foo=bar').type, '')
    t.assert.deepStrictEqual(Object.keys(safeParse('text/plain; profile=http://localhost; foo=bar').parameters).length, 0)

    t.assert.deepStrictEqual(safeParse('text/plain; profile=http://localhost').type, '')
    t.assert.deepStrictEqual(Object.keys(safeParse('text/plain; profile=http://localhost').parameters).length, 0)
  })

  await t.test('should return dummyContentType on missing argument', function (t) {
    t.plan(2)
    // @ts-expect-error should reject non-strings
    t.assert.deepStrictEqual(safeParse().type, '')
    // @ts-expect-error should reject non-strings
    t.assert.deepStrictEqual(Object.keys(safeParse().parameters).length, 0)
  })

  await t.test('should return dummyContentType on non-strings', function (t) {
    t.plan(2)
    // @ts-expect-error should reject non-strings
    t.assert.deepStrictEqual(safeParse(null).type, '')
    // @ts-expect-error should reject non-strings
    t.assert.deepStrictEqual(Object.keys(safeParse(null).parameters).length, 0)
  })
})
