import * as fs from 'fs';
export declare const chmod: typeof fs.promises.chmod, copyFile: typeof fs.promises.copyFile, lstat: typeof fs.promises.lstat, mkdir: typeof fs.promises.mkdir, open: typeof fs.promises.open, readdir: typeof fs.promises.readdir, rename: typeof fs.promises.rename, rm: typeof fs.promises.rm, rmdir: typeof fs.promises.rmdir, stat: typeof fs.promises.stat, symlink: typeof fs.promises.symlink, unlink: typeof fs.promises.unlink;
export declare const IS_WINDOWS: boolean;
/**
 * Custom implementation of readlink to ensure Windows junctions
 * maintain trailing backslash for backward compatibility with Node.js < 24
 *
 * In Node.js 20, Windows junctions (directory symlinks) always returned paths
 * with trailing backslashes. Node.js 24 removed this behavior, which breaks
 * code that relied on this format for path operations.
 *
 * This implementation restores the Node 20 behavior by adding a trailing
 * backslash to all junction results on Windows.
 */
export declare function readlink(fsPath: string): Promise<string>;
export declare const UV_FS_O_EXLOCK = 268435456;
export declare const READONLY: number;
export declare function exists(fsPath: string): Promise<boolean>;
export declare function isDirectory(fsPath: string, useStat?: boolean): Promise<boolean>;
/**
 * On OSX/Linux, true if path starts with '/'. On Windows, true for paths like:
 * \, \hello, \\hello\share, C:, and C:\hello (and corresponding alternate separator cases).
 */
export declare function isRooted(p: string): boolean;
/**
 * Best effort attempt to determine whether a file exists and is executable.
 * @param filePath    file path to check
 * @param extensions  additional file extensions to try
 * @return if file exists and is executable, returns the file path. otherwise empty string.
 */
export declare function tryGetExecutablePath(filePath: string, extensions: string[]): Promise<string>;
export declare function getCmdPath(): string;
