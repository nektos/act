'use strict'

const util = require('node:util')
const contentType = require('content-type')
const fastContentTypeParser = require('..')
const Benchmark = require('benchmark')
const { parseContentType } = require('busboy/lib/utils')
const suite = new Benchmark.Suite()

module.exports = function (str) {
  console.log(`\nBenchmarking: "${str}"`)
  suite
    .add('util#MIMEType', function () {
      new util.MIMEType(str) // eslint-disable-line no-new
    })
    .add('fast-content-type-parse#parse', function () {
      fastContentTypeParser.parse(str)
    })
    .add('fast-content-type-parse#safeParse', function () {
      fastContentTypeParser.safeParse(str)
    })
    .add('content-type#parse', function () {
      contentType.parse(str)
    })

  if (parseContentType(str) !== undefined) {
    suite.add('busboy#parseContentType', function () {
      parseContentType(str)
    })
  }
  suite
    .on('cycle', function (event) {
      console.log(String(event.target))
    })
    .on('complete', function () {
      console.log('Fastest is ' + this.filter('fastest').map('name'))
    })
    .run()
}
