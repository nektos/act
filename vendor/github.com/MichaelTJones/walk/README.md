walk
====

Fast parallel version of golang filepath.Walk()

Performs traversals in parallel so set GOMAXPROCS appropriately. Vaues of 8 to 16 seem to work best on my 
4-CPU plus 4 SMT pseudo-CPU MacBookPro. The result is about 4x-6x the traversal rate of the standard Walk().
The two are not identical since we are walking the file system in a tumult of asynchronous walkFunc calls by
a number of goroutines. So, take note of the following:

1. This walk honors all of the walkFunc error semantics but as multiple user-supplied walkFuncs may simultaneously encounter a traversal error or generate one to stop traversal, only the FIRST of these will be returned as the Walk() result. 

2. Further, since there may be a few files in flight at the instant of  error discovery, a few more walkFunc calls may happen after the first error-generating call has signaled its desire to stop. In general this is a non-issue but it could matter so pay attention when designing your walkFunc. (For example, if you accumulate results then you need to have your own means to know to stop accumulating once you signal an error.)

3. Because the walkFunc is called concurrently in multiple goroutines, it needs to be careful about what it does with external data to avoid collisions. Results may be printed using fmt, but generally the best plan is to send results over a channel or accumulate counts using a locked mutex.

These issues are illustrated/handled in the simple traversal programs supplied with walk. There is also a test file that is just the tests from filepath in the Go language's standard library. Walk passes these tests when run in single process mode, and passes most of them in concurrent mode (GOMAXPROCS > 1). The problem is not a real problem, but one of the test expecting a specific number of errors to be found based on presumed sequential traversals.

Copyright (c) 2016 Michael T Jones

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
