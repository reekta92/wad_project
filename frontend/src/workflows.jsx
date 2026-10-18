import { useEffect, useRef, useState } from 'react';
import { Icon } from './icons.jsx';
import { DownloadButton } from './downloads.jsx';
import { ReferenceSelect } from './management.jsx';

const dates = [{ name: 'date_from', label: 'From date', type: 'date' }, { name: 'date_to', label: 'To date', type: 'date' }];
const department = { name: 'department_id', label: 'Department ID', type: 'number' };
const personnel = { name: 'personnel_id', label: 'Personnel ID', type: 'number' };
const name = (row) => `${row.title} ${row.first_name} ${row.last_name}`.trim();
const session = (row) => `${row.date} · ${row.start_time.slice(0, 5)}–${row.end_time.slice(0, 5)}`;
const timestamp = (value) => value ? new Date(value).toLocaleString('en-GB', { timeZone: 'UTC', timeZoneName: 'short' }) : 'None';

export const workflowPages = {
  dashboard: { title: 'Dashboard', icon: 'calendar', description: 'Plan exams, assign invigilators, and publish the timetable.' },
  'reports/workload': {
    title: 'Workload', icon: 'account', description: 'Count assigned rooms per invigilator, including personnel with no duties.',
    filters: [department, personnel, ...dates], columns: [['Invigilator', name], ['Personnel ID', (r) => r.personnel_id], ['Department', (r) => r.department], ['Duties', (r) => r.duties]],
  },
  'reports/availability': {
    title: 'Availability', icon: 'clock', description: 'Check eligibility for one additional duty in each session. Existing duties and approved leave are included.',
    filters: [department, personnel, { name: 'session_id', label: 'Session ID', type: 'number' }, ...dates],
    columns: [['Invigilator', (r) => `${r.personnel} · #${r.personnel_id}`], ['Department', (r) => r.department], ['Session', session], ['Eligible', (r) => r.available ? 'Yes' : 'No'], ['Reason', (r) => r.reason]],
  },
  'audit-logs': {
    title: 'Audit log', icon: 'list', description: 'Read recorded changes and their before/after values. Date filters use UTC days.',
    filters: [{ name: 'actor_user_id', label: 'Actor user ID', type: 'number' }, { name: 'entity', label: 'Entity' }, { name: 'action', label: 'Action' }, ...dates],
    columns: [['Time (UTC)', (r) => timestamp(r.created_at)], ['Actor user ID', (r) => r.actor_user_id ?? 'System'], ['Entity', (r) => `${r.entity}${r.entity_id ? ` #${r.entity_id}` : ' (batch)'}`], ['Action', (r) => r.action]],
  },
  'me/duties': {
    title: 'My duties', icon: 'calendar', description: 'Your assigned exam rooms, including current assignments before publication. Times use Europe/Istanbul.',
    filters: [{ name: 'q', label: 'Course name or code' }, { name: 'course_id', label: 'Course ID', type: 'number' }, ...dates],
    columns: [['Course', (r) => `${r.course_code} · ${r.course_name}`], ['Department', (r) => r.department], ['Session', session], ['Room', (r) => r.classroom]],
  },
  'me/workload': { title: 'My workload', icon: 'account', description: 'Your assigned room count for the selected dates.', filters: dates },
};

export function Workflow({ page, account, navigate }) {
  const definition = workflowPages[page];
  const [query, setQuery] = useState('');
  const [result, setResult] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionError, setActionError] = useState('');
  const [notice, setNotice] = useState('');
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState(false);
  const [season, setSeason] = useState('all');
  const [scope, setScope] = useState('all');
  const [allocation, setAllocation] = useState('');
  const [modal, setModal] = useState(null);
  const [modalError, setModalError] = useState('');
  const dialog = useRef(null);
  const opener = useRef(null);
  const summary = page === 'me/workload';
  const dashboard = page === 'dashboard';
  const refresh = () => setRevision((value) => value + 1);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    account.call(`${page}${query ? `?${query}` : ''}`, { signal: controller.signal }).then((data) => {
      if (!dashboard && !summary && (!Array.isArray(data.data) || !Number.isSafeInteger(data.total) || data.total < 0 || !Number.isSafeInteger(data.page) || data.page < 1 || !Number.isSafeInteger(data.page_size) || data.page_size < 1)) throw new Error('The report returned incomplete information. Please try again.');
      if (!controller.signal.aborted) setResult(data);
    }).catch((failure) => { if (!controller.signal.aborted) setError(failure.message); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [page, query, account.call, revision, dashboard, summary]);

  useEffect(() => {
    if (!modal) return;
    dialog.current.showModal();
    dialog.current.querySelector('button')?.focus();
  }, [modal]);

  function close() {
    dialog.current.close();
    setModal(null);
    setModalError('');
  }

  async function mutate(path, method, body, message) {
    if (busy) return;
    setBusy(true);
    setActionError('');
    setModalError('');
    setNotice('');
    try {
      const value = await account.call(path, { method, ...(body === undefined ? {} : { body }) });
      if (modal) close();
      setNotice(typeof message === 'function' ? message(value) : message);
      refresh();
    } catch (failure) {
      if (modal) setModalError(failure.message); else setActionError(failure.message);
    } finally { setBusy(false); }
  }

  async function audit(record, button) {
    opener.current = button;
    setBusy(true);
    setActionError('');
    try { setModal(await account.request(`audit-logs/${record.id}`)); }
    catch (failure) { setActionError(failure.message); }
    finally { setBusy(false); }
  }

  function filter(event) {
    event.preventDefault();
    const params = new URLSearchParams(summary ? {} : { page: '1' });
    for (const [key, value] of new FormData(event.currentTarget)) if (String(value).trim()) params.set(key, String(value).trim());
    setQuery(params.toString());
  }

  function turnPage(number) {
    const params = new URLSearchParams(query);
    params.set('page', String(number));
    setQuery(params.toString());
  }

  const disabled = busy || loading || Boolean(error);
  return <section className="workflow" aria-labelledby="workflow-title">
    <div className="section-heading"><h2 id="workflow-title" tabIndex="-1">{dashboard ? 'Schedule overview' : definition.title}</h2><button className="button" disabled={busy || loading} onClick={refresh}><Icon name="refresh" />Refresh</button></div>
    {definition.filters && <form className="resource-filters" onSubmit={filter}>
      {definition.filters.map((field) => <div key={field.name}><label htmlFor={`report-${field.name}`}>{field.label}</label><input id={`report-${field.name}`} name={field.name} type={field.type || 'text'} min={field.type === 'number' ? 1 : undefined} max={field.type === 'number' ? Number.MAX_SAFE_INTEGER : undefined} maxLength="100" /></div>)}
      {!summary && <div><label htmlFor="report-page-size">Per page</label><select id="report-page-size" name="page_size" defaultValue="20"><option value="10">10 records</option><option value="20">20 records</option><option value="50">50 records</option><option value="100">100 records</option></select></div>}
      <button className="button primary" type="submit" disabled={loading || busy}>Apply filters</button>
    </form>}
    <p role="status" className="management-note">{loading ? 'Loading…' : error ? 'Information unavailable' : dashboard ? 'Overview updated.' : summary ? 'Workload updated.' : `${result?.total ?? 0} records found`}</p>
    {notice && <p role="status" className="success-message">{notice}</p>}
    {(error || actionError) && <div className="message error" role="alert"><p>{error || actionError}</p>{error && <button className="button" onClick={refresh}>Try again</button>}</div>}
    {!error && result && dashboard && <>
      <div className="record-card publication-panel"><div><h3>{result.publication.published ? 'Timetable published' : 'Timetable unpublished'}</h3><p>Publishing requires at least one exam and a staffed room allocation for every exam. Schedule changes hide it again.</p><p className="record-meta">Last publication change: {timestamp(result.publication.updated_at)}</p></div><button className="button primary" disabled={disabled} onClick={() => mutate('schedule/publication', 'PUT', { published: !result.publication.published }, result.publication.published ? 'Timetable unpublished.' : 'Timetable published.')}>{result.publication.published ? 'Unpublish timetable' : 'Publish timetable'}</button></div>
      <dl className="dashboard-totals">{Object.entries(result.totals).map(([key, value]) => <div key={key}><dt>{key.replaceAll('_', ' ')}</dt><dd>{value}</dd></div>)}</dl>
      <div className="resource-grid workflow-controls">
        <form className="record-card" onSubmit={(event) => { event.preventDefault(); void mutate('schedule/automatic-planning', 'POST', { season }, (r) => r.count ? `${r.count} exams planned.` : 'No unplanned courses in this season.'); }}>
          <h3>Plan exams</h3><p className="management-note">Fill unplanned courses in course order using the earliest valid session. Existing exams stay in place. If any course cannot fit, the entire batch is rolled back; another ordering may succeed.</p>
          <label htmlFor="planning-season">Season</label><select id="planning-season" value={season} onChange={(e) => setSeason(e.target.value)}><option value="all">All terms</option><option value="autumn">Autumn (odd terms)</option><option value="spring">Spring (even terms)</option></select><div className="record-actions"><button className="button primary" type="submit" disabled={disabled}>Plan unplanned courses</button></div>
        </form>
        <form className="record-card" onSubmit={(event) => { event.preventDefault(); void mutate('invigilator-assignments/automatic', 'POST', { scope, ...(scope === 'one' ? { allocation_id: Number(allocation) } : {}) }, (r) => r.count ? `${r.count} rooms staffed.` : 'All rooms are already staffed.'); }}>
          <h3>Assign invigilators</h3><p className="management-note">Fill unstaffed rooms, preferring the course department and then the lowest workload. Existing duties stay in place. The whole batch must pass availability rules.</p>
          <label htmlFor="staffing-scope">Staffing scope</label><select id="staffing-scope" value={scope} onChange={(e) => setScope(e.target.value)}><option value="all">All unstaffed rooms</option><option value="one">One allocation</option></select>
          {scope === 'one' && <div className="allocation-field"><label htmlFor="staffing-allocation">Exam room allocation</label><ReferenceSelect field={{ reference: 'allocations', required: true }} request={account.request} administrator value={allocation} id="staffing-allocation" required onChange={(e) => setAllocation(e.target.value)} /></div>}
          <div className="record-actions"><button className="button primary" type="submit" disabled={disabled}>Staff selected rooms</button></div>
        </form>
      </div>
      <div className="resource-grid dashboard-previews">{[
        ['unplanned_courses', 'Unplanned courses', 'courses', (r) => `${r.code} · ${r.name}`],
        ['pending_requests', 'Pending requests', 'unavailability-requests', (r) => `${r.personnel} · ${r.date} · ${r.type}`],
        ['unstaffed_rooms', 'Unstaffed rooms', 'exams', (r) => `${r.course_code} · ${r.classroom} · allocation #${r.id}`],
      ].map(([key, label, target, describe]) => <article key={key} className="record-card"><h3>{label} ({result.totals[key]})</h3><p className="record-meta">First five records</p>{result.previews[key].length ? <ul>{result.previews[key].map((row) => <li key={row.id}>{describe(row)}</li>)}</ul> : <p className="management-note">Nothing waiting.</p>}<button className="button" onClick={() => navigate(target)}>Open {label.toLowerCase()}</button></article>)}</div>
      <section className="record-card download-panel" aria-labelledby="downloads-title"><h3 id="downloads-title">Downloads</h3><p>Export the current timetable, or keep a private copy of the application database.</p><div className="record-actions"><DownloadButton account={account} /><DownloadButton account={account} backup /></div></section>
      <div className="record-card clear-panel"><h3>Clear schedule</h3><p>Remove every exam, room allocation, and duty. Courses, sessions, rooms, personnel, requests, and audit history are kept.</p><button className="button destructive" disabled={disabled} onClick={(event) => { opener.current = event.currentTarget; setModalError(''); setModal('clear'); }}>Clear schedule…</button></div>
    </>}
    {!loading && !error && result && summary && <article className="record-card"><h3>{name(result)}</h3><p className="record-meta">{result.department}</p><dl className="record-details"><div><dt>Assigned duties</dt><dd>{result.duties}</dd></div></dl></article>}
    {!loading && !error && result && definition.columns && (result.data.length ? <div className="report-scroll" role="region" aria-label={`${definition.title} results; scroll horizontally for more columns`} tabIndex="0"><table className="report-table"><caption className="sr-only">{definition.title} results</caption><thead><tr>{definition.columns.map(([label]) => <th key={label} scope="col">{label}</th>)}{page === 'audit-logs' && <th scope="col">Details</th>}</tr></thead><tbody>{result.data.map((row) => <tr key={row.id ?? `${row.personnel_id}/${row.session_id}`}>
      {definition.columns.map(([label, render]) => <td key={label}>{render(row)}</td>)}{page === 'audit-logs' && <td><button className="button" disabled={busy} onClick={(event) => audit(row, event.currentTarget)}>View change #{row.id}</button></td>}
    </tr>)}</tbody></table></div> : <div className="message"><h3>No matching records</h3><p>Try a different date range or filter.</p></div>)}
    {!loading && !error && result && definition.columns && <nav className="pagination" aria-label="Report results pages"><button className="button" aria-label="Previous page" disabled={busy || result.page <= 1} onClick={() => turnPage(result.page - 1)}><Icon name="previous" /><span className="page-label">Previous</span></button><span>Page {result.page} of {Math.max(1, Math.ceil(result.total / result.page_size))}</span><button className="button" aria-label="Next page" disabled={busy || result.page >= Math.ceil(result.total / result.page_size)} onClick={() => turnPage(result.page + 1)}><span className="page-label">Next</span><Icon name="next" /></button></nav>}
    <dialog ref={dialog} className="resource-dialog" aria-labelledby="workflow-dialog-title" onClose={() => { setModal(null); setModalError(''); if (opener.current?.isConnected && !opener.current.disabled) opener.current.focus(); else document.getElementById('workflow-title')?.focus(); }} onCancel={(event) => { if (busy) event.preventDefault(); else setModal(null); }}>
      {modal && <div className="dialog-content"><h2 id="workflow-dialog-title">{modal === 'clear' ? 'Clear the entire schedule?' : `Audit change #${modal.id}`}</h2>
        {modal === 'clear' ? <><p>All exams, room allocations, and duties will be permanently removed, and the timetable will be unpublished. Master data, requests, and audit history are kept.</p><p>You will need to plan and staff exams again before publishing.</p></> : <><dl className="record-details"><div><dt>Time (UTC)</dt><dd>{timestamp(modal.created_at)}</dd></div><div><dt>Actor user ID</dt><dd>{modal.actor_user_id ?? 'System'}</dd></div><div><dt>Change</dt><dd>{modal.entity} · {modal.action}</dd></div></dl><h3>Before</h3><pre className="audit-json" role="region" tabIndex="0" aria-label="Before values">{JSON.stringify(modal.old_value, null, 2)}</pre><h3>After</h3><pre className="audit-json" role="region" tabIndex="0" aria-label="After values">{JSON.stringify(modal.new_value, null, 2)}</pre></>}
        {modalError && <p className="field-error" role="alert">{modalError}</p>}
        <div className="dialog-actions"><button className="button" disabled={busy} onClick={close}>{modal === 'clear' ? 'Cancel' : 'Close'}</button>{modal === 'clear' && <button className="button destructive" disabled={busy} onClick={() => mutate('schedule', 'DELETE', undefined, 'Schedule cleared. Master data and request history kept.')}>{busy ? 'Clearing…' : 'Clear all exams and duties'}</button>}</div>
      </div>}
    </dialog>
  </section>;
}
