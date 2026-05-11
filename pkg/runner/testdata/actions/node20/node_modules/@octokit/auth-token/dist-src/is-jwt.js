const b64url = "(?:[a-zA-Z0-9_-]+)";
const sep = "\\.";
const jwtRE = new RegExp(`^${b64url}${sep}${b64url}${sep}${b64url}$`);
const isJWT = jwtRE.test.bind(jwtRE);
export {
  isJWT
};
