#!/usr/bin/env perl

my $_fmt;
$_fmt = "gofmt";
$_fmt = "cat -n" if "cat" eq ($ARGV[0] || "");

use strict;
use warnings;
use IO::File;

my $self = __PACKAGE__;

sub functionLabel ($) {
    return "$_[0]_function";
}

sub trim ($) {
    local $_ = shift;
    s/^\s*//, s/\s*$// for $_;
    return $_;
}

open my $fmt, "|-", "$_fmt" or die $!;

$fmt->print(<<_END_);
package otto

import (
    "math"
)

func _newContext(runtime *_runtime) {
@{[ join "\n", $self->newContext() ]}
}

func newConsoleObject(runtime *_runtime) *_object {
@{[ join "\n", $self->newConsoleObject() ]}
}
_END_

for (qw/int int8 int16 int32 int64 uint uint8 uint16 uint32 uint64 float32 float64/) {
    $fmt->print(<<_END_);

func toValue_$_(value $_) Value {
    return Value{
        kind: valueNumber,
        value: value,
    }
}
_END_
}

$fmt->print(<<_END_);

func toValue_string(value string) Value {
    return Value{
        kind: valueString,
        value: value,
    }
}

func toValue_string16(value []uint16) Value {
    return Value{
        kind: valueString,
        value: value,
    }
}

func toValue_bool(value bool) Value {
    return Value{
        kind: valueBoolean,
        value: value,
    }
}

func toValue_object(value *_object) Value {
    return Value{
        kind: valueObject,
        value: value,
    }
}
_END_

close $fmt;

sub newConsoleObject {
    my $self = shift;

    return
        $self->block(sub {
            my $class = "Console";
            my @got = $self->functionDeclare(
                $class,
                "log", 0,
                "debug:log", 0,
                "info:log", 0,
                "error", 0,
                "warn:error", 0,
                "dir", 0,
                "time", 0,
                "timeEnd", 0,
                "trace", 0,
                "assert", 0,
            );
            return
            "return @{[ $self->newObject(@got) ]}"
        }),
    ;
}

sub newContext {
    my $self = shift;
    return
        # ObjectPrototype
        $self->block(sub {
            my $class = "Object";
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classObject",
                undef,
                "prototypeValueObject",
            ),
        }),

        # FunctionPrototype
        $self->block(sub {
            my $class = "Function";
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classObject",
                ".ObjectPrototype",
                "prototypeValueFunction",
            ),
        }),

        # ObjectPrototype
        $self->block(sub {
            my $class = "Object";
            my @got = $self->functionDeclare(
                $class,
                "valueOf", 0,
                "toString", 0,
                "toLocaleString", 0,
                "hasOwnProperty", 1,
                "isPrototypeOf", 1,
                "propertyIsEnumerable", 1,
            );
            my @propertyMap = $self->propertyMap(
                @got,
                $self->property("constructor", undef),
            );
            my $propertyOrder = $self->propertyOrder(@propertyMap);
            $propertyOrder =~ s/^propertyOrder: //;
            return
            ".${class}Prototype.property =", @propertyMap,
            ".${class}Prototype.propertyOrder =", $propertyOrder,
        }),

        # FunctionPrototype
        $self->block(sub {
            my $class = "Function";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
                "apply", 2,
                "call", 1,
                "bind", 1,
            );
            my @propertyMap = $self->propertyMap(
                @got,
                $self->property("constructor", undef),
                $self->property("length", $self->numberValue(0), "0"),
            );
            my $propertyOrder = $self->propertyOrder(@propertyMap);
            $propertyOrder =~ s/^propertyOrder: //;
            return
            ".${class}Prototype.property =", @propertyMap,
            ".${class}Prototype.propertyOrder =", $propertyOrder,
        }),

        # Object
        $self->block(sub {
            my $class = "Object";
            return
            ".$class =",
            $self->globalFunction(
                $class,
                1,
                $self->functionDeclare(
                    $class,
                    "getPrototypeOf", 1,
                    "getOwnPropertyDescriptor", 2,
                    "defineProperty", 3,
                    "defineProperties", 2,
                    "create", 2,
                    "isExtensible", 1,
                    "preventExtensions", 1,
                    "isSealed", 1,
                    "seal", 1,
                    "isFrozen", 1,
                    "freeze", 1,
                    "keys", 1,
                    "getOwnPropertyNames", 1,
                ),
            ),
        }),

        # Function
        $self->block(sub {
            my $class = "Function";
            return
            "Function :=",
            $self->globalFunction(
                $class,
                1,
            ),
            ".$class = Function",
        }),

        # Array
        $self->block(sub {
            my $class = "Array";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
                "toLocaleString", 0,
                "concat", 1,
                "join", 1,
                "splice", 2,
                "shift", 0,
                "pop", 0,
                "push", 1,
                "slice", 2,
                "unshift", 1,
                "reverse", 0,
                "sort", 1,
                "indexOf", 1,
                "lastIndexOf", 1,
                "every", 1,
                "some", 1,
                "forEach", 1,
                "map", 1,
                "filter", 1,
                "reduce", 1,
                "reduceRight", 1,
            );
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classArray",
                ".ObjectPrototype",
                undef,
                $self->property("length", $self->numberValue("uint32(0)"), "0100"),
                @got,
            ),
            ".$class =",
            $self->globalFunction(
                $class,
                1,
                $self->functionDeclare(
                    $class,
                    "isArray", 1,
                ),
            ),
        }),

        # String
        $self->block(sub {
            my $class = "String";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
                "valueOf", 0,
                "charAt", 1,
                "charCodeAt", 1,
                "concat", 1,
                "indexOf", 1,
                "lastIndexOf", 1,
                "match", 1,
                "replace", 2,
                "search", 1,
                "split", 2,
                "slice", 2,
                "substring", 2,
                "toLowerCase", 0,
                "toUpperCase", 0,
                "substr", 2,
                "trim", 0,
                "trimLeft", 0,
                "trimRight", 0,
                "localeCompare", 1,
                "toLocaleLowerCase", 0,
                "toLocaleUpperCase", 0,
            );
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classString",
                ".ObjectPrototype",
                "prototypeValueString",
                $self->property("length", $self->numberValue("int(0)"), "0"),
                @got,
            ),
            ".$class =",
            $self->globalFunction(
                $class,
                1,
                $self->functionDeclare(
                    $class,
		            "fromCharCode", 1,
                ),
            ),
        }),

        # Boolean
        $self->block(sub {
            my $class = "Boolean";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
                "valueOf", 0,
            );
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classObject",
                ".ObjectPrototype",
                "prototypeValueBoolean",
                @got,
            ),
            ".$class =",
            $self->globalFunction(
                $class,
                1,
                $self->functionDeclare(
                    $class,
                ),
            ),
        }),

        # Number
        $self->block(sub {
            my $class = "Number";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
                "valueOf", 0,
                "toFixed", 1,
                "toExponential", 1,
                "toPrecision", 1,
                "toLocaleString", 1,
            );
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classObject",
                ".ObjectPrototype",
                "prototypeValueNumber",
                @got,
            ),
            ".$class =",
            $self->globalFunction(
                $class,
                1,
                $self->functionDeclare(
                    $class,
                    "isNaN", 1,
                ),
                $self->numberConstantDeclare(
                    "MAX_VALUE", "math.MaxFloat64",
                    "MIN_VALUE", "math.SmallestNonzeroFloat64",
                    "NaN", "math.NaN()",
                    "NEGATIVE_INFINITY", "math.Inf(-1)",
                    "POSITIVE_INFINITY", "math.Inf(+1)",
                ),
            ),
        }),

        # Math
        $self->block(sub {
            my $class = "Math";
            return
            ".$class =",
            $self->globalObject(
                $class,
                $self->functionDeclare(
                    $class,
                    "abs", 1,
                    "acos", 1,
                    "asin", 1,
                    "atan", 1,
                    "atan2", 1,
                    "ceil", 1,
                    "cos", 1,
                    "exp", 1,
                    "floor", 1,
                    "log", 1,
                    "max", 2,
                    "min", 2,
                    "pow", 2,
                    "random", 0,
                    "round", 1,
                    "sin", 1,
                    "sqrt", 1,
                    "tan", 1,
                ),
                $self->numberConstantDeclare(
                    "E", "math.E",
                    "LN10", "math.Ln10",
                    "LN2", "math.Ln2",
                    "LOG2E", "math.Log2E",
                    "LOG10E", "math.Log10E",
                    "PI", "math.Pi",
                    "SQRT1_2", "sqrt1_2",
                    "SQRT2", "math.Sqrt2",
                )
            ),
        }),

        # Date
        $self->block(sub {
            my $class = "Date";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
                "toDateString", 0,
                "toTimeString", 0,
                "toUTCString", 0,
                "toISOString", 0,
                "toJSON", 1,
                "toGMTString", 0,
                "toLocaleString", 0,
                "toLocaleDateString", 0,
                "toLocaleTimeString", 0,
                "valueOf", 0,
                "getTime", 0,
                "getYear", 0,
                "getFullYear", 0,
                "getUTCFullYear", 0,
                "getMonth", 0,
                "getUTCMonth", 0,
                "getDate", 0,
                "getUTCDate", 0,
                "getDay", 0,
                "getUTCDay", 0,
                "getHours", 0,
                "getUTCHours", 0,
                "getMinutes", 0,
                "getUTCMinutes", 0,
                "getSeconds", 0,
                "getUTCSeconds", 0,
                "getMilliseconds", 0,
                "getUTCMilliseconds", 0,
                "getTimezoneOffset", 0,
                "setTime", 1,
                "setMilliseconds", 1,
                "setUTCMilliseconds", 1,
                "setSeconds", 2,
                "setUTCSeconds", 2,
                "setMinutes", 3,
                "setUTCMinutes", 3,
                "setHours", 4,
                "setUTCHours", 4,
                "setDate", 1,
                "setUTCDate", 1,
                "setMonth", 2,
                "setUTCMonth", 2,
                "setYear", 1,
                "setFullYear", 3,
                "setUTCFullYear", 3,
            );
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classObject",
                ".ObjectPrototype",
                "prototypeValueDate",
                @got,
            ),
            ".$class =",
            $self->globalFunction(
                $class,
                7,
                $self->functionDeclare(
                    $class,
                    "parse", 1,
                    "UTC", 7,
                    "now", 0,
                ),
            ),
        }),

        # RegExp
        $self->block(sub {
            my $class = "RegExp";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
                "exec", 1,
                "test", 1,
                "compile", 1,
            );
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classObject",
                ".ObjectPrototype",
                "prototypeValueRegExp",
                @got,
            ),
            ".$class =",
            $self->globalFunction(
                $class,
                2,
                $self->functionDeclare(
                    $class,
                ),
            ),
        }),

        # Error
        $self->block(sub {
            my $class = "Error";
            my @got = $self->functionDeclare(
                $class,
                "toString", 0,
            );
            return
            ".${class}Prototype =",
            $self->globalPrototype(
                $class,
                "_classObject",
                ".ObjectPrototype",
                undef,
                @got,
                $self->property("name", $self->stringValue("Error")),
                $self->property("message", $self->stringValue("")),
            ),
            ".$class =",
            $self->globalFunction(
                $class,
                1,
                $self->functionDeclare(
                    $class,
                ),
            ),
        }),

        (map {
            my $class = "${_}Error";
            $self->block(sub {
                my @got = $self->functionDeclare(
                    $class,
                );
                return
                ".${class}Prototype =",
                $self->globalPrototype(
                    $class,
                    "_classObject",
                    ".ErrorPrototype",
                    undef,
                    @got,
                    $self->property("name", $self->stringValue($class)),
                ),
                ".$class =",
                $self->globalFunction(
                    $class,
                    1,
                    $self->functionDeclare(
                        $class,
                    ),
                ),
            });
        } qw/Eval Type Range Reference Syntax URI/),

        # JSON
        $self->block(sub {
            my $class = "JSON";
            return
            ".$class =",
            $self->globalObject(
                $class,
                $self->functionDeclare(
                    $class,
                    "parse", 2,
                    "stringify", 3,
                ),
            ),
        }),

        # Global
        $self->block(sub {
            my $class = "Global";
            my @got = $self->functionDeclare(
                $class,
                "eval", 1,
                "parseInt", 2,
                "parseFloat", 1,
                "isNaN", 1,
                "isFinite", 1,
                "decodeURI", 1,
                "decodeURIComponent", 1,
                "encodeURI", 1,
                "encodeURIComponent", 1,
                "escape", 1,
                "unescape", 1,
            );
            my @propertyMap = $self->propertyMap(
                @got,
                $self->globalDeclare(
                    "Object",
                    "Function",
                    "Array",
                    "String",
                    "Boolean",
                    "Number",
                    "Math",
                    "Date",
                    "RegExp",
                    "Error",
                    "EvalError",
                    "TypeError",
                    "RangeError",
                    "ReferenceError",
                    "SyntaxError",
                    "URIError",
                    "JSON",
                ),
                $self->property("undefined", $self->undefinedValue(), "0"),
                $self->property("NaN", $self->numberValue("math.NaN()"), "0"),
                $self->property("Infinity", $self->numberValue("math.Inf(+1)"), "0"),
            );
            my $propertyOrder = $self->propertyOrder(@propertyMap);
            $propertyOrder =~ s/^propertyOrder: //;
            return
            "runtime.globalObject.property =",
            @propertyMap,
            "runtime.globalObject.propertyOrder =",
            $propertyOrder,
            ;
        }),
    ;
}

sub propertyMap {
    my $self = shift;
    return "map[string]_property{", (join ",\n", @_, ""), "}",
}

our (@preblock, @postblock);
sub block {
    my $self = shift;
    local @preblock = ();
    local @postblock = ();
    my @input = $_[0]->();
    my @output;
    while (@input) {
        local $_ = shift @input;
        if (m/^\./) {
            $_ = "runtime.global$_";
        }
        if (m/ :?=$/) {
            $_ .= shift @input;
        }
        push @output, $_;
    }
    return
    "{",
        @preblock,
        @output,
        @postblock,
    "}",
    ;
}

sub numberConstantDeclare {
    my $self = shift;
    my @got;
    while (@_) {
        my $name = shift;
        my $value = shift;
        push @got, $self->property($name, $self->numberValue($value), "0"),
    }
    return @got;
}

sub functionDeclare {
    my $self = shift;
    my $class = shift;
    my $builtin = "builtin${class}";
    my @got;
    while (@_) {
        my $name = shift;
        my $length = shift;
        $name = $self->newFunction($name, "${builtin}_", $length);
        push @got, $self->functionProperty($name),
    }
    return @got;
}

sub globalDeclare {
    my $self = shift;
    my @got;
    while (@_) {
        my $name = shift;
        push @got, $self->property($name, $self->objectValue("runtime.global.$name"), "0101"),
    }
    return @got;
}

sub propertyOrder {
    my $self = shift;
    my $propertyMap = join "", @_;

    my (@keys) = $propertyMap =~ m/("\w+"):/g;
    my $propertyOrder =
        join "\n", "propertyOrder: []string{", (join ",\n", @keys, ""), "}";
    return $propertyOrder;
}

sub globalObject {
    my $self = shift;
    my $name = shift;

    my $propertyMap = "";
    if (@_) {
        $propertyMap = join "\n", $self->propertyMap(@_);
        my $propertyOrder = $self->propertyOrder($propertyMap);
        $propertyMap = "property: $propertyMap,\n$propertyOrder,";
    }

    return trim <<_END_;
&_object{
    runtime: runtime,
    class: "$name",
    objectClass: _classObject,
    prototype: runtime.global.ObjectPrototype,
    extensible: true,
    $propertyMap
}
_END_
}

sub globalFunction {
    my $self = shift;
    my $name = shift;
    my $length = shift;

    my $builtin = "builtin${name}";
    my $builtinNew = "builtinNew${name}";
    my $prototype = "runtime.global.${name}Prototype";
    my $propertyMap = "";
    unshift @_,
        $self->property("length", $self->numberValue($length), "0"),
        $self->property("prototype", $self->objectValue($prototype), "0"),
    ;

    if (@_) {
        $propertyMap = join "\n", $self->propertyMap(@_);
        my $propertyOrder = $self->propertyOrder($propertyMap);
        $propertyMap = "property: $propertyMap,\n$propertyOrder,";
    }

    push @postblock, $self->statement(
        "$prototype.property[\"constructor\"] =",
        $self->property(undef, $self->objectValue("runtime.global.${name}"), "0101"),
    );

    return trim <<_END_;
&_object{
    runtime: runtime,
    class: "Function",
    objectClass: _classObject,
    prototype: runtime.global.FunctionPrototype,
    extensible: true,
    value: @{[ $self->nativeFunctionOf($name, $builtin, $builtinNew) ]},
    $propertyMap
}
_END_
}

sub nativeCallFunction {
    my $self = shift;
    my $name = shift;
    my $func = shift;
    return trim <<_END_;
_nativeCallFunction{ "$name", $func }
_END_
}

sub globalPrototype {
    my $self = shift;
    my $class = shift;
    my $classObject = shift;
    my $prototype = shift;
    my $value = shift;

    if (!defined $prototype) {
        $prototype = "nil";
    }

    if (!defined $value) {
        $value = "nil";
    }

    if ($prototype =~ m/^\./) {
        $prototype = "runtime.global$prototype";
    }

    my $propertyMap = "";
    if (@_) {
        $propertyMap = join "\n", $self->propertyMap(@_);
        my $propertyOrder = $self->propertyOrder($propertyMap);
        $propertyMap = "property: $propertyMap,\n$propertyOrder,";
    }

    return trim <<_END_;
&_object{
    runtime: runtime,
    class: "$class",
    objectClass: $classObject,
    prototype: $prototype,
    extensible: true,
    value: $value,
    $propertyMap
}
_END_
}

sub newFunction {
    my $self = shift;
    my $name = shift;
    my $func = shift;
    my $length = shift;

    my @name = ($name, $name);
    if ($name =~ m/^(\w+):(\w+)$/) {
        @name = ($1, $2);
        $name = $name[0];
    }

    if ($func =~ m/^builtin\w+_$/) {
        $func = "$func$name[1]";
    }

    my $propertyOrder = "";
    my @propertyMap = (
        $self->property("length", $self->numberValue($length), "0"),
    );

    if (@propertyMap) {
        $propertyOrder = $self->propertyOrder(@propertyMap);
        $propertyOrder = "$propertyOrder,";
    }

    my $label = functionLabel($name);
    push @preblock, $self->statement(
        "$label := @{[ trim <<_END_ ]}",
&_object{
    runtime: runtime,
    class: "Function",
    objectClass: _classObject,
    prototype: runtime.global.FunctionPrototype,
    extensible: true,
    property: @{[ join "\n", $self->propertyMap(@propertyMap) ]},
    $propertyOrder
    value: @{[ $self->nativeFunctionOf($name, $func) ]},
}
_END_
    );

    return $name;
}

sub newObject {
    my $self = shift;

    my $propertyMap = join "\n", $self->propertyMap(@_);
    my $propertyOrder = $self->propertyOrder($propertyMap);

    return trim <<_END_;
&_object{
    runtime: runtime,
    class: "Object",
    objectClass: _classObject,
    prototype: runtime.global.ObjectPrototype,
    extensible: true,
    property: $propertyMap,
    $propertyOrder,
}
_END_
}

sub newPrototypeObject {
    my $self = shift;
    my $class = shift;
    my $objectClass = shift;
    my $value = shift;
    if (defined $value) {
        $value = "value: $value,";
    }

    my $propertyMap = join "\n", $self->propertyMap(@_);
    my $propertyOrder = $self->propertyOrder($propertyMap);

    return trim <<_END_;
&_object{
    runtime: runtime,
    class: "$class",
    objectClass: $objectClass,
    prototype: runtime.global.ObjectPrototype,
    extensible: true,
    property: $propertyMap,
    $propertyOrder,
    $value
}
_END_
}

sub functionProperty {
    my $self = shift;
    my $name = shift;

    return $self->property(
        $name,
        $self->objectValue(functionLabel($name))
    );
}

sub statement {
    my $self = shift;
    return join "\n", @_;
}

sub functionOf {
    my $self = shift;
    my $call = shift;
    my $construct = shift;
    if ($construct) {
        $construct = "construct: $construct,";
    } else {
        $construct = "";
    }

    return trim <<_END_
_functionObject{
    call: $call,
    $construct
}
_END_
}

sub nativeFunctionOf {
    my $self = shift;
    my $name = shift;
    my $call = shift;
    my $construct = shift;
    if ($construct) {
        $construct = "construct: $construct,";
    } else {
        $construct = "";
    }

    return trim <<_END_
_nativeFunctionObject{
    name: "$name",
    call: $call,
    $construct
}
_END_
}

sub nameProperty {
    my $self = shift;
    my $name = shift;
    my $value = shift;

    return trim <<_END_;
"$name": _property{
    mode: 0101,
    value: $value,
}
_END_
}

sub numberValue {
    my $self = shift;
    my $value = shift;
    return trim <<_END_;
Value{
    kind: valueNumber,
    value: $value,
}
_END_
}

sub property {
    my $self = shift;
    my $name = shift;
    my $value = shift;
    my $mode = shift;
    $mode = "0101" unless defined $mode;
    if (! defined $value) {
        $value = "Value{}";
    }
    if (defined $name) {
        return trim <<_END_;
"$name": _property{
    mode: $mode,
    value: $value,
}
_END_
    } else {
        return trim <<_END_;
_property{
    mode: $mode,
    value: $value,
}
_END_
    }

}

sub objectProperty {
    my $self = shift;
    my $name = shift;
    my $value = shift;

    return trim <<_END_;
"$name": _property{
    mode: 0101,
    value: @{[ $self->objectValue($value)]},
}
_END_
}

sub objectValue {
    my $self = shift;
    my $value = shift;
    return trim <<_END_
Value{
    kind: valueObject,
    value: $value,
}
_END_
}

sub stringValue {
    my $self = shift;
    my $value = shift;
    return trim <<_END_
Value{
    kind: valueString,
    value: "$value",
}
_END_
}

sub booleanValue {
    my $self = shift;
    my $value = shift;
    return trim <<_END_
Value{
    kind: valueBoolean,
    value: $value,
}
_END_
}

sub undefinedValue {
    my $self = shift;
    return trim <<_END_
Value{
    kind: valueUndefined,
}
_END_
}
