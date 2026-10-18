// Compatibility surface for integrations that imported the v0.1.0 client.
// New views import their endpoint-specific module from web/src/api/ instead.
export {
  APIRequestError,
  api,
  clearCSRF,
  getAuthStatus as authStatus,
  jsonBody as withBody,
  setCSRF
} from './api/client'
