.PHONY: test test-race test-release release release-check test-262
.PHONY: parser
.PHONY: otto assets underscore

TESTS := \
	~

TEST := -v --run
TEST := -v
TEST := -v --run Test\($(subst $(eval) ,\|,$(TESTS))\)
TEST := .

test: parser inline.go
	go test -i
	go test $(TEST)
	@echo PASS

parser:
	$(MAKE) -C parser

inline.go: inline.pl
	./$< > $@

#################
# release, test #
#################

release: test-race test-release
	for package in . parser token ast file underscore registry; do (cd $$package && godocdown --signature > README.markdown); done
	@echo \*\*\* make release-check
	@echo PASS

release-check: .test
	$(MAKE) -C test build test
	$(MAKE) -C .test/test262 build test
	@echo PASS

test-262: .test
	$(MAKE) -C .test/test262 build test
	@echo PASS

test-release:
	go test -i
	go test

test-race:
	go test -race -i
	go test -race

#################################
# otto, assets, underscore, ... #
#################################

otto:
	$(MAKE) -C otto

assets:
	mkdir -p .assets
	for file in underscore/test/*.js; do tr "\`" "_" < $$file > .assets/`basename $$file`; done

underscore:
	$(MAKE) -C $@

