import { useRef, useState } from 'react';
import { Icon } from './icons.jsx';

export function DownloadButton({ account, query = '', backup = false }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const dialog = useRef(null);
  const button = useRef(null);
  const label = backup ? 'Download database backup' : account.profile.role === 'administrator' ? 'Export current XLSX' : 'Download XLSX';

  async function download() {
    if (busy) return;
    setBusy(true);
    setError('');
    setNotice('');
    const filters = new URLSearchParams(query);
    filters.delete('page'); filters.delete('page_size');
    try {
      const blob = await account.download(backup ? 'backups' : `schedule/export${filters.size ? `?${filters}` : ''}`, { method: backup ? 'POST' : 'GET' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = backup ? 'application.dump' : 'exam-schedule.xlsx';
      document.body.append(link); link.click(); link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      if (backup) dialog.current.close();
      setNotice('Download ready. Check your browser’s downloads.');
    } catch (failure) { setError(failure.message); }
    finally { setBusy(false); }
  }

  function open() {
    setError(''); setNotice('');
    dialog.current.showModal();
    dialog.current.querySelector('button').focus();
  }

  return <div className="download-control">
    <button ref={button} className="button tonal" disabled={busy} onClick={backup ? open : download}><Icon name={backup ? 'download' : 'calendar'} />{busy ? 'Preparing download…' : label}</button>
    <p className="record-meta">{backup ? 'Application data only. Keep this file private.' : 'All matching exams, across all pages.'}</p>
    {!backup && error && <p className="field-error" role="alert">{error}</p>}
    {notice && <p className="record-meta" role="status">{notice}</p>}
    {backup && <dialog ref={dialog} aria-labelledby="backup-dialog-title" onCancel={(event) => { if (busy) event.preventDefault(); }} onClose={() => button.current?.focus()}><div className="dialog-content">
      <h2 id="backup-dialog-title">Download a private database backup?</h2><p>This file includes personnel, leave requests, account links, and audit history. Store it securely. It does not include sign-in credentials or the identity service.</p>
      {busy && <p role="status">Preparing your backup. Keep this window open.</p>}
      {error && <p role="alert" className="field-error">{error}</p>}
      <div className="dialog-actions"><button className="button" disabled={busy} onClick={() => dialog.current.close()}>Cancel</button><button className="button primary" disabled={busy} onClick={download}>{busy ? 'Preparing…' : 'Download private backup'}</button></div>
    </div></dialog>}
  </div>;
}
