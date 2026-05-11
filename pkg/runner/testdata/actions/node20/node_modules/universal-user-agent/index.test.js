import { getUserAgent } from "./index.js";

if (getUserAgent instanceof Function === false) {
  throw new Error("getUserAgent is not a function");
}

if (typeof getUserAgent() !== "string") {
  throw new Error("getUserAgent does not return a string");
}

if ("Deno" in globalThis) {
  if (/Deno\//.test(getUserAgent()) === false) {
    throw new Error(
      "getUserAgent does not return the correct user agent for Deno"
    );
  }
} else if ("Bun" in globalThis) {
  if (/Bun\//.test(getUserAgent()) === false) {
    throw new Error(
      "getUserAgent does not return the correct user agent for Bun"
    );
  }
} else {
  if (/Node\.js\//.test(getUserAgent()) === false) {
    throw new Error(
      "getUserAgent does not return the correct user agent for Node.js"
    );
  }
}

delete globalThis.navigator;
delete globalThis.process;

if (getUserAgent() !== "<environment undetectable>") {
  throw new Error(
    "getUserAgent does not return the correct user agent for undetectable environment"
  );
}

console.info("getUserAgent test passed");
