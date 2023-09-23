## Releases

## 1.0.10

Contains a bug fix where proxy is defined without a user and password. see [PR here](https://github.com/actions/http-client/pull/42)   

## 1.0.9
Throw HttpClientError instead of a generic Error from the \<verb>Json() helper methods when the server responds with a non-successful status code. 

## 1.0.8
Fixed security issue where a redirect (e.g. 302) to another domain would pass headers.  The fix was to strip the authorization header if the hostname was different.  More [details in PR #27](https://github.com/actions/http-client/pull/27)

## 1.0.7
Update NPM dependencies and add 429 to the list of HttpCodes

## 1.0.6
Automatically sends Content-Type and Accept application/json headers for \<verb>Json() helper methods if not set in the client or parameters.

## 1.0.5
Adds \<verb>Json() helper methods for json over http scenarios.

## 1.0.4
Started to add \<verb>Json() helper methods.  Do not use this release for that.  Use >= 1.0.5 since there was an issue with types.

## 1.0.1 to 1.0.3
Adds proxy support.
