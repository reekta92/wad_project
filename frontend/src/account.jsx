import { useEffect, useState } from 'react';
import { authentication } from './auth.js';
import { fetchProfile } from './api.js';
import { Icon } from './icons.jsx';

export function useAccount() {
  const [client, setClient] = useState(null);
  const [profile, setProfile] = useState(null);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState('');
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    let adapter;
    let revision = 0;
    setBusy(true);
    setError('');
    authentication().then(async (value) => {
      if (controller.signal.aborted) return;
      adapter = value;
      setClient(value);
      const clear = () => { revision++; setProfile(null); };
      const expired = () => {
        value.clearToken();
        setError('Your session ended. Sign in again to continue.');
      };
      const loadProfile = async () => {
        const current = ++revision;
        try {
          const result = await fetchProfile(value.token, controller.signal);
          if (!controller.signal.aborted && current === revision && value.authenticated) {
            setProfile(result);
            setError('');
          }
        } catch (failure) {
          if (!controller.signal.aborted && current === revision) {
            setProfile(null);
            if (failure.status === 401) value.clearToken();
            setError(failure.message);
          }
        }
      };
      value.onAuthLogout = clear;
      value.onAuthRefreshError = expired;
      value.onAuthRefreshSuccess = loadProfile;
      if (value.authenticated) {
        await value.updateToken(30);
        await loadProfile();
      }
    }).catch(() => {
      if (!controller.signal.aborted) setError('Sign-in is temporarily unavailable. You can still browse the published schedule.');
    }).finally(() => {
      if (!controller.signal.aborted) setBusy(false);
    });
    return () => {
      controller.abort();
      if (adapter) {
        adapter.onAuthLogout = undefined;
        adapter.onAuthRefreshError = undefined;
        adapter.onAuthRefreshSuccess = undefined;
      }
    };
  }, [attempt]);

  async function act(action) {
    setError('');
    setBusy(true);
    try { await action(); } catch {
      setError('The sign-in service could not complete this action. Please try again.');
    } finally { setBusy(false); }
  }

  const redirectUri = `${window.location.origin}/`;
  return {
    profile, busy, error, available: Boolean(client), authenticated: Boolean(client?.authenticated),
    retry: () => setAttempt((value) => value + 1),
    signIn: () => act(() => client.login({ redirectUri, prompt: 'login' })),
    registerPasskey: () => act(() => client.login({ redirectUri, prompt: 'login', action: 'webauthn-register-passwordless' })),
    signOut: () => act(() => {
      const url = client.createLogoutUrl({ redirectUri });
      client.clearToken();
      window.location.assign(url);
    }),
  };
}

export function AccountPanel({ account }) {
  const profile = account.profile;
  const role = profile?.role === 'administrator' ? 'Administrator'
    : profile?.permissions.includes('invigilator') ? 'Invigilator' : 'Student member';
  return <section id="account" className="account-panel" aria-labelledby="account-title" aria-busy={account.busy}>
    <span className="account-symbol"><Icon name={profile ? 'check' : 'account'} /></span>
    <div className="account-copy"><h2 id="account-title">{profile ? role : account.authenticated ? 'Account access' : 'Guest access'}</h2>
      {profile ? <p>Signed in as <strong>{profile.username}</strong>.</p>
        : <p>The timetable is open to everyone. Sign in to access your faculty profile.</p>}
      {account.error && <p className="account-error" role="alert">{account.error}</p>}
    </div>
    <div className="account-actions">
      {!account.available && !account.busy && <button className="button" onClick={account.retry}>Retry sign-in</button>}
      {account.authenticated && <button className="button" disabled={account.busy} onClick={account.registerPasskey}>Register a passkey</button>}
      {account.authenticated && !profile && <button className="button" disabled={account.busy} onClick={account.signIn}>Sign in again</button>}
    </div>
  </section>;
}
