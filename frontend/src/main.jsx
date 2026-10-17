import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { fetchSchedule } from './api.js';
import { AccountPanel, useAccount } from './account.jsx';
import './style.css';

function CalendarIcon() {
  return <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="3" /><path d="M7 2v6m10-6v6M3 11h18m-14 5 3 3 6-6" /></svg>;
}

const clock = (value) => value.slice(0, 5);
const dateLabel = (value) => new Intl.DateTimeFormat('en-GB', { dateStyle: 'full', timeZone: 'Europe/Istanbul' }).format(new Date(`${value}T12:00:00+03:00`));

function Exam({ exam }) {
  return <article className="exam-card">
    <div className="exam-heading"><span className="course-code">{exam.course_code}</span><span className="term">Term {exam.term}</span></div>
    <h3>{exam.course_name}</h3>
    <p className="department">{exam.department}</p>
    <dl>
      <div><dt>Date</dt><dd>{dateLabel(exam.date)}</dd></div>
      <div><dt>Time</dt><dd>{clock(exam.start_time)}–{clock(exam.end_time)}</dd></div>
      <div><dt>Room</dt><dd>{exam.rooms.map((room) => room.name).join(', ') || 'Not allocated'}</dd></div>
    </dl>
  </article>;
}

function Calendar({ exams }) {
  const dates = [...new Set(exams.map((exam) => exam.date))].sort();
  const times = [...new Set(exams.map((exam) => `${exam.start_time}/${exam.end_time}`))].sort();
  return <div className="calendar-scroll" role="region" aria-label="Calendar; scroll horizontally for more dates" tabIndex="0">
    <table className="calendar-table">
      <caption className="sr-only">Published exams on the current results page</caption>
      <thead><tr><th scope="col">Session</th>{dates.map((date) => <th key={date} scope="col">{dateLabel(date)}</th>)}</tr></thead>
      <tbody>{times.map((time) => <tr key={time}>
        <th scope="row">{time.split('/').map(clock).join('–')}</th>
        {dates.map((date) => <td key={date}>{exams.filter((exam) => exam.date === date && `${exam.start_time}/${exam.end_time}` === time).map((exam) => <Exam key={exam.id} exam={exam} />)}</td>)}
      </tr>)}</tbody>
    </table>
  </div>;
}

function App() {
  const account = useAccount();
  const initial = new URLSearchParams(window.location.search);
  const [query, setQuery] = useState(window.location.search.slice(1));
  const [search, setSearch] = useState(initial.get('q') || '');
  const [from, setFrom] = useState(initial.get('date_from') || '');
  const [to, setTo] = useState(initial.get('date_to') || '');
  const [pageSize, setPageSize] = useState(initial.get('page_size') || '20');
  const [view, setView] = useState('list');
  const [menuOpen, setMenuOpen] = useState(false);
  const [result, setResult] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [retry, setRetry] = useState(0);
  const dialog = useRef(null);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    fetchSchedule(query, controller.signal).then((data) => {
      if (!controller.signal.aborted) setResult(data);
    }).catch((failure) => {
      if (!controller.signal.aborted) setError(failure.message);
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [query, retry]);

  function navigate(params) {
    const value = params.toString();
    window.history.replaceState(null, '', value ? `?${value}#schedule` : '/#schedule');
    setQuery(value);
  }

  function filter(event) {
    event.preventDefault();
    const params = new URLSearchParams({ page: '1', page_size: pageSize });
    if (search.trim()) params.set('q', search.trim());
    if (from) params.set('date_from', from);
    if (to) params.set('date_to', to);
    navigate(params);
  }

  function page(number) {
    const params = new URLSearchParams(query);
    params.set('page', String(number));
    navigate(params);
  }

  return <>
    <a className="skip-link" href="#schedule">Skip to schedule</a>
    <header className="site-header">
      <div className="header-inner mx-auto flex max-w-6xl items-center justify-between gap-4">
        <a className="brand" href="/" aria-label="Faculty Exams home"><CalendarIcon /><span>Faculty<span className="brand-light">Exams</span></span></a>
        <button className="menu-toggle" aria-expanded={menuOpen} aria-controls="main-navigation" onClick={() => setMenuOpen(!menuOpen)}>{menuOpen ? 'Close menu' : 'Menu'}</button>
        <nav id="main-navigation" className={menuOpen ? 'navigation open' : 'navigation'} aria-label="Main navigation">
          <a href="#schedule" onClick={() => setMenuOpen(false)}>Exam schedule</a>
          <button onClick={() => { setMenuOpen(false); dialog.current.showModal(); }}>Schedule information</button>
          <a href="#account" onClick={() => setMenuOpen(false)}>My account</a>
          {account.authenticated && <button disabled={account.busy} onClick={account.signOut}>Sign out</button>}
        </nav>
      </div>
    </header>

    <main className="mx-auto max-w-6xl">
      <AccountPanel account={account} />
      <section className="hero" aria-labelledby="page-title">
        <div><p className="eyebrow">Faculty exam timetable</p><h1 id="page-title">Your exams.<br /><span>Clearly planned.</span></h1><p className="hero-copy">Find your course, check the time, and know where to be. Browse the published schedule below.</p><a className="button primary" href="#schedule">Find an exam <span aria-hidden="true">↗</span></a></div>
        <img src="/campus.svg" alt="Illustration of a faculty building beside an exam calendar" width="520" height="300" />
      </section>

      <section id="schedule" className="schedule-section" aria-labelledby="schedule-title">
        <div className="section-heading"><div><p className="eyebrow">Plan your exam days</p><h2 id="schedule-title">Published schedule</h2></div><p className="timezone">All times: Europe/Istanbul</p></div>
        <form className="filter-form" onSubmit={filter}>
          <div className="search-field"><label htmlFor="course-search">Course name or code</label><input id="course-search" type="search" maxLength="100" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="For example, Web or WAD301" /></div>
          <div><label htmlFor="date-from">From date</label><input id="date-from" type="date" value={from} onChange={(event) => setFrom(event.target.value)} /></div>
          <div><label htmlFor="date-to">To date</label><input id="date-to" type="date" min={from || undefined} value={to} onChange={(event) => setTo(event.target.value)} /></div>
          <div><label htmlFor="page-size">Per page</label><select id="page-size" value={pageSize} onChange={(event) => setPageSize(event.target.value)}><option value="10">10 exams</option><option value="20">20 exams</option><option value="50">50 exams</option></select></div>
          <button className="button primary" type="submit" disabled={loading}>Apply filters</button>
        </form>

        <div className="results-toolbar"><p role="status" aria-live="polite">{loading ? 'Loading published exams…' : error ? 'Schedule unavailable' : `${result?.total ?? 0} exams found`}</p><div className="view-switch" aria-label="Schedule display"><button aria-pressed={view === 'list'} onClick={() => setView('list')}>List</button><button aria-pressed={view === 'calendar'} onClick={() => setView('calendar')}>Calendar</button></div></div>
        {error && <div className="message error" role="alert"><h3>Could not load the schedule</h3><p>{error}</p><button className="button" onClick={() => setRetry(retry + 1)}>Try again</button></div>}
        {!loading && !error && result?.data.length === 0 && <div className="message"><CalendarIcon /><h3>No published exams match these filters</h3><p>Try a different course or date range.</p></div>}
        {!loading && !error && result?.data.length > 0 && (view === 'calendar' ? <Calendar exams={result.data} /> : <div className="exam-grid">{result.data.map((exam) => <Exam key={exam.id} exam={exam} />)}</div>)}
        {!loading && !error && result && <nav className="pagination" aria-label="Schedule results pages"><button className="button" disabled={result.page <= 1} onClick={() => page(result.page - 1)}>Previous</button><span>Page {result.page} of {Math.max(1, Math.ceil(result.total / result.page_size))}</span><button className="button" disabled={result.page >= Math.ceil(result.total / result.page_size)} onClick={() => page(result.page + 1)}>Next</button></nav>}
      </section>
    </main>

    <footer className="site-footer"><div className="footer-inner mx-auto max-w-6xl"><div><a className="brand" href="/"><CalendarIcon /><span>Faculty Exams</span></a><p>Faculty examination information.</p></div><div><h2>Schedule</h2><a href="#schedule">Browse published exams</a><button onClick={() => dialog.current.showModal()}>Using this schedule</button></div><div><h2>Project</h2><a href="https://github.com/reekta92/wad_project">Source repository</a><p>Times shown in Europe/Istanbul.</p></div></div></footer>

    <dialog ref={dialog} aria-labelledby="information-title"><div className="dialog-content"><p className="eyebrow">Before your exam</p><h2 id="information-title">Using the published schedule</h2><p>Search by course name or code, then narrow the date range. Switch between list and calendar views; each view shows the current page of results.</p><p>Times use the faculty timezone, Europe/Istanbul. Only published exams appear here. Check the schedule again before your exam in case dates or rooms change.</p><button className="button primary" autoFocus onClick={() => dialog.current.close()}>Got it</button></div></dialog>
  </>;
}

createRoot(document.getElementById('root')).render(<React.StrictMode><App /></React.StrictMode>);
