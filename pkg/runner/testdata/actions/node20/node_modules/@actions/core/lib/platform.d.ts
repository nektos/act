export declare const platform: NodeJS.Platform;
export declare const arch: NodeJS.Architecture;
export declare const isWindows: boolean;
export declare const isMacOS: boolean;
export declare const isLinux: boolean;
export declare function getDetails(): Promise<{
    name: string;
    platform: string;
    arch: string;
    version: string;
    isWindows: boolean;
    isMacOS: boolean;
    isLinux: boolean;
}>;
