import Keycloak from 'keycloak-js';

let initialization;

// One adapter initialization survives React StrictMode; tokens remain in adapter memory.
export function authentication() {
  if (!initialization) initialization = initialize().catch((error) => {
    initialization = undefined;
    throw error;
  });
  return initialization;
}

async function initialize() {
  const response = await fetch('/api/v1/auth/config', { signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error('Sign-in is temporarily unavailable. You can still browse the published schedule.');
  const config = await response.json();
  if (typeof config.url !== 'string' || typeof config.realm !== 'string' || config.client_id !== 'exam-web') {
    throw new Error('Sign-in is temporarily unavailable. Please try again later.');
  }
  const client = new Keycloak({ url: config.url, realm: config.realm, clientId: config.client_id });
  // Keycloak schedules the initial expiry timer only when this callback already exists.
  client.onTokenExpired = () => {
    void client.updateToken(30).catch(() => {
      if (client.authenticated) client.clearToken();
    });
  };
  await client.init({ pkceMethod: 'S256', checkLoginIframe: false, responseMode: 'fragment', flow: 'standard', scope: 'openid' });
  return client;
}
