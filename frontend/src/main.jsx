import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { fetchSchedule } from './api.js';
import { AccountPanel, useAccount } from './account.jsx';
import { Icon } from './icons.jsx';
import { Management } from './management.jsx';
import { resources } from './resources.js';
import { DownloadButton } from './downloads.jsx';
import { Workflow, workflowPages } from './workflows.jsx';

const pages = { ...resources, ...workflowPages };
import './style.css';

const clock = (value) => value.slice(0, 5);
const dateLabel = (value) => new Intl.DateTimeFormat('en-GB', { dateStyle: 'full', timeZone: 'Europe/Istanbul' }).format(new Date(`${value}T12:00:00+03:00`));

function Exam({ exam }) {
  return <article className={`exam-card accent-${['blue', 'green', 'purple'][exam.term % 3]}`}>
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
  const [resource, setResource] = useState(null);
  const [theme, setTheme] = useState(() => document.documentElement.dataset.theme || 'system');
  const [result, setResult] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [retry, setRetry] = useState(0);
  const dialog = useRef(null);
  const menuButton = useRef(null);
  const permitted = account.profile?.role === 'administrator' ? ['dashboard', ...Object.keys(resources), 'reports/workload', 'reports/availability', 'audit-logs'] : account.profile?.permissions.includes('invigilator') ? ['me/duties', 'me/workload', 'unavailability-requests'] : [];
  const activeResource = permitted.includes(resource) ? resource : null;

  function showResource(value) {
    setResource(value);
    setMenuOpen(false);
  }

  useEffect(() => {
    if (activeResource) {
      document.getElementById('page-title')?.focus();
      window.scrollTo({ top: 0 });
    }
  }, [activeResource]);

  useEffect(() => {
    if (!menuOpen) return;
    const close = (event) => {
      if (event.key === 'Escape') {
        setMenuOpen(false);
        menuButton.current.focus();
      }
    };
    document.addEventListener('keydown', close);
    return () => document.removeEventListener('keydown', close);
  }, [menuOpen]);

  function changeTheme(event) {
    const value = event.target.value;
    document.documentElement.dataset.theme = value;
    setTheme(value);
    try { localStorage.setItem('exams-theme', value); } catch { /* The current page still uses the selected theme. */ }
  }

  useEffect(() => {
    if (activeResource) return;
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
  }, [query, retry, activeResource]);

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
    <a className="skip-link" href="#main-content">Skip to content</a>
    <header className="site-header">
      <div className="header-inner">
        <div className="header-brand">
          <button ref={menuButton} className="icon-button menu-toggle" aria-label={menuOpen ? 'Close menu' : 'Open menu'} aria-expanded={menuOpen} aria-controls="main-navigation" onClick={() => setMenuOpen(!menuOpen)}><Icon name={menuOpen ? 'close' : 'menu'} /></button>
          <a className="brand" href="/" aria-label="Faculty Exams home"><span className="brand-mark"><Icon name="calendar" /></span><span><span className="brand-faculty">Faculty</span><span className="brand-light">Exams</span></span></a>
        </div>
        <div className="header-actions">
          <div className="theme-control"><Icon name="appearance" /><select aria-label="Color theme" value={theme} onChange={changeTheme}><option value="system">Device theme</option><option value="light">Light</option><option value="dark">Dark</option></select></div>
          <a className="icon-button account-shortcut" href="#account" aria-label="My account"><Icon name="account" /></a>
          <button className={account.authenticated ? 'button tonal' : 'button primary'} disabled={account.busy || !account.available} onClick={account.authenticated ? account.signOut : account.signIn}>{account.authenticated ? 'Sign out' : 'Sign in'}</button>
        </div>
      </div>
    </header>

    <div className="workspace">
      <nav id="main-navigation" className={menuOpen ? 'navigation open' : 'navigation'} aria-label="Main navigation">
        <a className="button compose-button" href="#course-search" onClick={() => showResource(null)}><Icon name="search" />Find an exam</a>
        <div className="nav-links">
          <a className={activeResource ? 'nav-link' : 'nav-link selected'} href="#schedule" onClick={() => showResource(null)}><Icon name="calendar" />Exam schedule</a>
          <a className="nav-link" href="#account" onClick={() => setMenuOpen(false)}><Icon name="account" />My account</a>
          <button className="nav-link" onClick={() => { setMenuOpen(false); dialog.current.showModal(); }}><Icon name="help" />Schedule information</button>
        </div>
        {permitted.length > 0 && <div className="nav-links business-navigation"><p className="nav-section-label">{account.profile.role === 'administrator' ? 'Administration' : 'My workspace'}</p>{permitted.map((key) => <button key={key} className={activeResource === key ? 'nav-link selected' : 'nav-link'} aria-current={activeResource === key ? 'page' : undefined} onClick={() => showResource(key)}><Icon name={pages[key].icon} />{key === 'unavailability-requests' ? account.profile.role === 'administrator' ? 'Requests' : 'My requests' : key === 'invigilator-assignments' ? 'Assignments' : pages[key].title}</button>)}</div>}
        <div className="sidebar-note"><Icon name="clock" /><p>One timetable.<br />A little less to remember.</p><span>Europe/Istanbul</span></div>
      </nav>

      <main id="main-content" className="content-surface" tabIndex="-1">
        <section className="page-intro" aria-labelledby="page-title">
          <div><p className="eyebrow">Faculty / {activeResource ? 'Workspace' : 'Examinations'}</p><h1 id="page-title" tabIndex="-1">{activeResource ? pages[activeResource].title : 'A clearer exam day.'}</h1><p className="intro-copy">{activeResource ? pages[activeResource].description : 'Find your course. Check the time. You’re all set.'}</p></div>
          {!activeResource && <span className="publication-chip"><span className="status-dot" />Published timetable</span>}
        </section>
        <AccountPanel account={account} />

      {activeResource ? workflowPages[activeResource] ? <Workflow key={activeResource} page={activeResource} account={account} navigate={showResource} /> : <Management key={activeResource} resource={activeResource} account={account} /> : <section id="schedule" className="schedule-section" aria-labelledby="schedule-title">
        <div className="section-heading"><h2 id="schedule-title">Published schedule</h2><p className="timezone"><Icon name="clock" />Europe/Istanbul</p></div>
        <form className="filter-form" onSubmit={filter}>
          <div className="search-field"><label htmlFor="course-search">Course name or code</label><div className="search-input"><Icon name="search" /><input id="course-search" type="search" maxLength="100" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search your courses" /></div></div>
          <div><label htmlFor="date-from">From date</label><input id="date-from" type="date" value={from} onChange={(event) => setFrom(event.target.value)} /></div>
          <div><label htmlFor="date-to">To date</label><input id="date-to" type="date" min={from || undefined} value={to} onChange={(event) => setTo(event.target.value)} /></div>
          <div><label htmlFor="page-size">Per page</label><select id="page-size" value={pageSize} onChange={(event) => setPageSize(event.target.value)}><option value="10">10 exams</option><option value="20">20 exams</option><option value="50">50 exams</option></select></div>
          <button className="button primary apply-button" type="submit" disabled={loading}><Icon name="search" />Apply filters</button>
        </form>

        <div className="results-toolbar"><p role="status" aria-live="polite">{loading ? 'Loading published exams…' : error ? 'Schedule unavailable' : `${result?.total ?? 0} exam${result?.total === 1 ? '' : 's'} found`}</p>{account.profile && <DownloadButton account={account} query={query} />}<div className="view-switch" role="group" aria-label="Schedule display"><button aria-pressed={view === 'list'} onClick={() => setView('list')}><Icon name="list" />List</button><button aria-pressed={view === 'calendar'} onClick={() => setView('calendar')}><Icon name="calendar" />Calendar</button></div></div>
        {error && <div className="message error" role="alert"><h3>Could not load the schedule</h3><p>{error}</p><button className="button" onClick={() => setRetry(retry + 1)}>Try again</button></div>}
        {!loading && !error && result?.data.length === 0 && <div className="message"><img src="/campus.svg" alt="" width="160" height="120" /><h3>No published exams match these filters</h3><p>Try a different course or date range.</p></div>}
        {!loading && !error && result?.data.length > 0 && (view === 'calendar' ? <Calendar exams={result.data} /> : <div className="exam-grid">{result.data.map((exam) => <Exam key={exam.id} exam={exam} />)}</div>)}
        {!loading && !error && result && <nav className="pagination" aria-label="Schedule results pages"><button className="button" aria-label="Previous page" disabled={result.page <= 1} onClick={() => page(result.page - 1)}><Icon name="previous" /><span className="page-label">Previous</span></button><span>Page {result.page} of {Math.max(1, Math.ceil(result.total / result.page_size))}</span><button className="button" aria-label="Next page" disabled={result.page >= Math.ceil(result.total / result.page_size)} onClick={() => page(result.page + 1)}><span className="page-label">Next</span><Icon name="next" /></button></nav>}
      </section>}
      </main>
    </div>

    <footer className="site-footer"><div className="footer-inner"><div><a className="brand" href="/"><Icon name="calendar" /><span>Faculty Exams</span></a><p>A little more clarity for exam season.</p></div><div><h2>Schedule</h2><a href="#schedule" onClick={() => showResource(null)}>Browse published exams</a><button onClick={() => dialog.current.showModal()}>Using this schedule</button></div><div><h2>Project</h2><a href="https://github.com/reekta92/wad_project">Source repository <Icon name="arrow" /></a><p>All times: Europe/Istanbul.</p></div></div></footer>

    <dialog ref={dialog} aria-labelledby="information-title"><div className="dialog-content"><p className="eyebrow">Before your exam</p><h2 id="information-title">Using the published schedule</h2><p>Search by course name or code, then narrow the date range. Switch between list and calendar views; each view shows the current page of results.</p><p>Times use the faculty timezone, Europe/Istanbul. Only published exams appear here. Check the schedule again before your exam in case dates or rooms change.</p><button className="button primary" autoFocus onClick={() => dialog.current.close()}>Got it</button></div></dialog>
  </>;
}

createRoot(document.getElementById('root')).render(<React.StrictMode><App /></React.StrictMode>);
