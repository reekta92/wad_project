import { useEffect, useRef, useState } from 'react';
import { Icon } from './icons.jsx';
import { resources, resourceFields, resourceBody } from './resources.js';

function optionLabel(resource, value) {
  if (resource === 'allocations') return value.name;
  return `${resources[resource].heading(value)} · #${value.id}`;
}

function ReferenceSelect({ field, request, value, ...props }) {
  const [page, setPage] = useState(1);
  const [options, setOptions] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    const source = field.reference === 'allocations' ? 'exams' : field.reference;
    request(`${source}?page=${page}&page_size=100`, { signal: controller.signal }).then((result) => {
      if (controller.signal.aborted) return;
      const entries = field.reference === 'allocations' ? result.data.flatMap((exam) => exam.rooms.map((room) => ({ id: room.id, name: `${exam.course_code} · ${room.name} · ${exam.date} · #${room.id}` }))) : result.data;
      setOptions((previous) => page === 1 ? entries : [...previous, ...entries]);
      setTotal(result.total);
    }).catch((failure) => { if (!controller.signal.aborted) setError(failure.message); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [field.reference, request, page, retry]);
  return <>
    <select {...props} value={value ?? ''} disabled={props.disabled || loading}>
      <option value="">{loading ? 'Loading options…' : field.required ? 'Choose a record' : 'None'}</option>
      {value && !options.some((option) => String(option.id) === String(value)) && <option value={value}>Current record #{value}</option>}
      {options.map((option) => <option key={option.id} value={option.id}>{optionLabel(field.reference, option)}</option>)}
    </select>
    {error && <p className="field-error" role="alert">{error} <button type="button" className="text-button" onClick={() => setRetry(retry + 1)}>Retry options</button></p>}
    {page * 100 < total && <button className="text-button" type="button" disabled={loading} onClick={() => setPage(page + 1)}>Load more options</button>}
  </>;
}

function Field({ field, value, onChange, id, error, request, administrator, editing }) {
  const props = {
    id, name: field.name, value: value ?? '', required: field.required,
    disabled: editing && field.immutable,
    'aria-invalid': error ? 'true' : undefined,
    'aria-describedby': error || field.help ? `${id}-help` : undefined,
    onChange: (event) => onChange(field.type === 'checkbox' ? event.target.checked : event.target.value),
  };
  const reference = field.reference && (administrator || field.reference !== 'sessions');
  let control;
  if (reference) control = <ReferenceSelect field={field} request={request} {...props} />;
  else if (field.type === 'textarea') control = <textarea {...props} maxLength={field.maxLength} rows="3" />;
  else if (field.options) control = <select {...props}><option value="">Choose an option</option>{field.options.map((option) => <option key={option} value={option}>{option}</option>)}</select>;
  else if (field.type === 'checkbox') control = <input {...props} type="checkbox" checked={Boolean(value)} />;
  else control = <input {...props} type={field.type || 'text'} maxLength={field.maxLength} min={field.type === 'number' ? field.min ?? 1 : undefined} max={field.type === 'number' ? field.max ?? (field.name.endsWith('_id') || field.name === 'id' ? Number.MAX_SAFE_INTEGER : 2147483647) : undefined} step={field.type === 'number' || field.type === 'time' ? 1 : undefined} />;
  return <div className={field.type === 'checkbox' ? 'field checkbox-field' : field.type === 'textarea' ? 'field wide-field' : 'field'}>
    <label htmlFor={id}>{field.label}{field.required && <span aria-hidden="true"> *</span>}</label>
    {control}
    {(error || field.help) && <p id={`${id}-help`} className={error ? 'field-error' : 'field-help'}>{error || field.help}</p>}
  </div>;
}

function RecordDetails({ record, fields }) {
  return <dl className="record-details">
    <div><dt>ID</dt><dd>{record.id}</dd></div>
    {fields.filter((field) => field.name !== 'id').map((field) => <div key={field.name}><dt>{field.label}</dt><dd>{record[field.name] == null ? 'None' : typeof record[field.name] === 'boolean' ? record[field.name] ? 'Yes' : 'No' : String(record[field.name])}</dd></div>)}
    {record.department && <div><dt>Department name</dt><dd>{record.department}</dd></div>}
    {record.personnel && <div><dt>Invigilator</dt><dd>{record.personnel}</dd></div>}
    {record.status && <div><dt>Status</dt><dd>{record.status}</dd></div>}
    {record.decided_at && <div><dt>Decided at</dt><dd>{record.decided_at}</dd></div>}
    {record.rooms && <div><dt>Allocated rooms</dt><dd>{record.rooms.map((room) => <p key={room.id}>{room.name} · allocation #{room.id} · {room.capacity} seats</p>)}</dd></div>}
  </dl>;
}

export function Management({ resource, account }) {
  const definition = resources[resource];
  const administrator = account.profile.role === 'administrator';
  const [query, setQuery] = useState('page=1&page_size=20');
  const [result, setResult] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [revision, setRevision] = useState(0);
  const [editor, setEditor] = useState(null);
  const [values, setValues] = useState({});
  const [formError, setFormError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const dialog = useRef(null);
  const addButton = useRef(null);
  const fields = resourceFields(resource, administrator, editor?.mode !== 'create');

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    account.request(`${resource}?${query}`, { signal: controller.signal }).then((data) => {
      if (!controller.signal.aborted) setResult(data);
    }).catch((failure) => { if (!controller.signal.aborted) setError(failure); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [resource, account.request, query, revision]);

  useEffect(() => {
    if (!editor) return;
    if (!dialog.current.open) dialog.current.showModal();
    dialog.current.querySelector('input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled])')?.focus();
  }, [editor?.mode, editor?.record?.id]);

  useEffect(() => { if (notice) addButton.current?.focus(); }, [notice]);

  function close() {
    dialog.current.close();
    setEditor(null);
    setFormError(null);
  }

  async function open(mode, record) {
    setNotice('');
    setFormError(null);
    if (mode === 'create') {
      setValues(Object.fromEntries(resourceFields(resource, administrator, false).map((field) => [field.name, field.initial ?? ''])));
      setEditor({ mode, record: null });
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const value = await account.request(`${resource}/${record.id}`);
      setValues(value);
      setEditor({ mode, record: value });
    } catch (failure) { setError(failure); } finally { setBusy(false); }
  }

  async function submit(event) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setFormError(null);
    try {
      const create = editor.mode === 'create';
      const path = create ? resource : `${resource}/${editor.record.id}`;
      const method = editor.mode === 'delete' ? 'DELETE' : create ? 'POST' : 'PUT';
      await account.request(path, { method, ...(method === 'DELETE' ? {} : { body: resourceBody(fields, values) }) });
      close();
      setNotice(method === 'DELETE' ? 'Record deleted.' : 'Changes saved.');
      setRevision((value) => value + 1);
    } catch (failure) { setFormError(failure); } finally { setBusy(false); }
  }

  function filter(event) {
    event.preventDefault();
    const params = new URLSearchParams({ page: '1' });
    for (const [name, value] of new FormData(event.currentTarget)) if (String(value).trim()) params.set(name, String(value).trim());
    setQuery(params.toString());
  }

  function page(number) {
    const params = new URLSearchParams(query);
    params.set('page', String(number));
    setQuery(params.toString());
  }

  const mutable = (record) => resource !== 'unavailability-requests' || record.status === 'pending';
  const formFields = fields.map((field) => resource === 'unavailability-requests' && field.name === 'date' ? { ...field, required: !values.session_id } : field);
  return <section id="management" className="management" aria-labelledby="management-title">
    <div className="section-heading"><h2 id="management-title">Manage records</h2><button ref={addButton} className="button primary" disabled={busy} onClick={() => open('create')}><Icon name="add" />Add {definition.singular}</button></div>
    {administrator && <p className="management-note">Changes to courses, rooms, sessions, personnel, exams, or assignments can hide the public timetable until it is published again.</p>}
    <form className="resource-filters" onSubmit={filter}>
      {definition.filters.filter((field) => administrator || !field.adminOnly).map((field) => <div key={field.name}><label htmlFor={`filter-${field.name}`}>{field.label}</label>{field.options
        ? <select id={`filter-${field.name}`} name={field.name}><option value="">Any</option>{field.options.map((value) => <option key={value} value={value}>{value}</option>)}</select>
        : <input id={`filter-${field.name}`} name={field.name} type={field.type || 'search'} min={field.type === 'number' ? field.min ?? 1 : undefined} max={field.max} maxLength="100" />}</div>)}
      <div><label htmlFor="resource-page-size">Per page</label><select id="resource-page-size" name="page_size" defaultValue="20"><option value="10">10 records</option><option value="20">20 records</option><option value="50">50 records</option><option value="100">100 records</option></select></div>
      <button className="button primary" type="submit" disabled={loading}>Apply filters</button>
    </form>
    <div className="results-toolbar"><p role="status">{loading ? 'Loading records…' : error ? 'Records unavailable' : `${result?.total ?? 0} records found`}</p><button className="button" disabled={loading || busy} onClick={() => setRevision((value) => value + 1)}><Icon name="refresh" />Refresh</button></div>
    {notice && <p role="status" className="success-message">{notice}</p>}
    {error && <div className="message error" role="alert"><h3>Could not complete this action</h3><p>{error.message}</p><button className="button" onClick={() => setRevision((value) => value + 1)}>Try again</button></div>}
    {!loading && !error && result?.data.length === 0 && <div className="message"><Icon name={definition.icon} /><h3>No matching records</h3><p>Change the filters or add a {definition.singular}.</p></div>}
    {!loading && !error && <div className="resource-grid">{result?.data.map((record) => <article key={record.id} className="record-card">
      <div className="record-heading"><span className="record-id">#{record.id}</span>{record.status && <span className="course-code">{record.status}</span>}</div>
      <h3>{definition.heading(record)}</h3>
      <p className="record-meta">{record.department || record.personnel || record.description || record.reason || (record.date ? `${record.date} · session #${record.session_id}` : record.capacity ? `${record.capacity} seats · floor ${record.floor} · ${record.active ? 'available' : 'inactive'}` : '')}</p>
      {record.rooms && <p className="record-meta">{record.rooms.map((room) => room.name).join(', ')}</p>}
      <div className="record-actions"><button className="button" disabled={busy} onClick={() => open('detail', record)}>Details</button><button className="button tonal" disabled={busy || !mutable(record)} onClick={() => open('edit', record)}><Icon name="edit" />Edit</button><button className="icon-button delete-button" aria-label={`Delete ${definition.singular} #${record.id}`} disabled={busy || !mutable(record)} onClick={() => open('delete', record)}><Icon name="delete" /></button></div>
    </article>)}</div>}
    {!loading && !error && result && <nav className="pagination" aria-label="Resource results pages"><button className="button" aria-label="Previous page" disabled={result.page <= 1 || busy} onClick={() => page(result.page - 1)}><Icon name="previous" /><span className="page-label">Previous</span></button><span>Page {result.page} of {Math.max(1, Math.ceil(result.total / result.page_size))}</span><button className="button" aria-label="Next page" disabled={result.page >= Math.ceil(result.total / result.page_size) || busy} onClick={() => page(result.page + 1)}><span className="page-label">Next</span><Icon name="next" /></button></nav>}
    <dialog ref={dialog} className="resource-dialog" aria-labelledby="record-dialog-title" onClose={() => setEditor(null)} onCancel={(event) => { if (busy) event.preventDefault(); else setEditor(null); }}>
      {editor && <div className="dialog-content"><p className="eyebrow">{definition.title}{editor.record ? ` / #${editor.record.id}` : ''}</p><h2 id="record-dialog-title">{editor.mode === 'create' ? `Add ${definition.singular}` : editor.mode === 'edit' ? `Edit ${definition.singular}` : editor.mode === 'delete' ? `Delete ${definition.singular}?` : 'Record details'}</h2>
        {editor.mode === 'detail' ? <><RecordDetails record={editor.record} fields={fields} /><div className="dialog-actions"><button className="button" onClick={close}>Close</button>{mutable(editor.record) && <button className="button primary" onClick={() => setEditor({ ...editor, mode: 'edit' })}>Edit</button>}</div></>
          : <form className="resource-form" onSubmit={submit} aria-busy={busy}>
            {editor.mode === 'delete' ? <p className="wide-field">Delete <strong>{definition.heading(editor.record)}</strong>? Referenced records must be removed from their relationships first. Deleting an exam also removes its room allocations and assignments.</p>
              : <><p className="form-help wide-field">Fields marked * are required. All times use Europe/Istanbul.</p>{formFields.map((field) => <Field key={field.name} field={field} id={`record-${field.name}`} value={values[field.name]} error={formError?.fields?.[field.name]} administrator={administrator} editing={editor.mode === 'edit'} request={account.request} onChange={(value) => setValues((previous) => ({ ...previous, [field.name]: value }))} />)}</>}
            {formError && <div className="message error wide-field" role="alert"><p>{formError.message}</p></div>}
            <div className="dialog-actions wide-field"><button className="button" type="button" disabled={busy} onClick={close}>Cancel</button><button className={editor.mode === 'delete' ? 'button destructive' : 'button primary'} type="submit" disabled={busy}>{busy ? 'Saving…' : editor.mode === 'delete' ? 'Delete record' : 'Save changes'}</button></div>
          </form>}
      </div>}
    </dialog>
  </section>;
}
