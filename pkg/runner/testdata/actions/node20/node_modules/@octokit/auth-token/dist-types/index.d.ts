import type { StrategyInterface, Token, Authentication } from "./types.js";
export type Types = {
    StrategyOptions: Token;
    AuthOptions: never;
    Authentication: Authentication;
};
export declare const createTokenAuth: StrategyInterface;
