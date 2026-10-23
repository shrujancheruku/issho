import { useState, type FormEvent } from 'react';

import { supabase } from './lib';

/** Email-code sign-in, as in the app. Only existing accounts can sign in here. */
export function SignIn() {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const send = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    // shouldCreateUser: false — admins are existing accounts given the
    // role; this page never creates one.
    const { error } = await supabase.auth.signInWithOtp({ email: email.trim(), options: { shouldCreateUser: false } });
    setBusy(false);
    if (error) setError(error.message);
    else setSent(true);
  };

  const verify = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    const { error } = await supabase.auth.verifyOtp({ email: email.trim(), token: code.trim(), type: 'email' });
    setBusy(false);
    if (error) setError(error.message);
  };

  return (
    <main className="center">
      <form className="card narrow" onSubmit={sent ? verify : send}>
        <h1><span className="brand">ISSHO</span> admin</h1>
        {!sent ? (
          <>
            <label>
              Email
              <input type="email" autoComplete="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
            </label>
            <button disabled={busy}>Send code</button>
          </>
        ) : (
          <>
            <p className="muted">We sent a 6-digit code to {email}.</p>
            <label>
              Code
              <input inputMode="numeric" autoComplete="one-time-code" required value={code} onChange={(e) => setCode(e.target.value)} />
            </label>
            <div className="row">
              <button disabled={busy}>Sign in</button>
              <button type="button" className="secondary" onClick={() => { setSent(false); setCode(''); }}>Use another email</button>
            </div>
          </>
        )}
        {error && <p className="error">{error}</p>}
      </form>
    </main>
  );
}
