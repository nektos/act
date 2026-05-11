import * as path from 'path';
/**
 * toPosixPath converts the given path to the posix form. On Windows, \\ will be
 * replaced with /.
 *
 * @param pth. Path to transform.
 * @return string Posix path.
 */
export function toPosixPath(pth) {
    return pth.replace(/[\\]/g, '/');
}
/**
 * toWin32Path converts the given path to the win32 form. On Linux, / will be
 * replaced with \\.
 *
 * @param pth. Path to transform.
 * @return string Win32 path.
 */
export function toWin32Path(pth) {
    return pth.replace(/[/]/g, '\\');
}
/**
 * toPlatformPath converts the given path to a platform-specific path. It does
 * this by replacing instances of / and \ with the platform-specific path
 * separator.
 *
 * @param pth The path to platformize.
 * @return string The platform-specific path.
 */
export function toPlatformPath(pth) {
    return pth.replace(/[/\\]/g, path.sep);
}
//# sourceMappingURL=path-utils.js.map