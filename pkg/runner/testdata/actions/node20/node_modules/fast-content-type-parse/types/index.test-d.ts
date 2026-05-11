import { parse, safeParse, defaultContentType } from '..'
import { expectType, expectError } from 'tsd'

expectError(parse())
expectError(parse(null))
expectError(parse(123))

expectType<string>(parse('string').type)
expectType<Record<string, string>>(parse('string').parameters)

expectError(safeParse())
expectError(safeParse(null))
expectError(safeParse(123))

expectType<string>(safeParse('string').type)
expectType<Record<string, string>>(safeParse('string').parameters)

expectType<string>(defaultContentType.type)
expectType<Record<string, string>>(defaultContentType.parameters)
