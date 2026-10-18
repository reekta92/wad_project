export async function fetchSchedule(query, signal) {
  const response = await fetch(`/api/v1/schedule${query ? `?${query}` : ''}`, {
    headers: { Accept: 'application/json' },
    signal,
  });
  let body;
  try {
    body = await response.json();
  } catch {
    throw new Error('The schedule service returned an unreadable response. Please try again.');
  }
  if (!response.ok) {
    throw new Error(body.error?.message || `The schedule could not be loaded (${response.status}).`);
  }
  if (!Array.isArray(body.data) || !body.data.every(validExam) || !Number.isSafeInteger(body.total) || body.total < 0
    || !Number.isSafeInteger(body.page) || body.page < 1
    || !Number.isSafeInteger(body.page_size) || body.page_size < 1) {
    throw new Error('The schedule service returned incomplete information. Please try again.');
  }
  return body;
}

function validExam(exam) {
  return exam && Number.isSafeInteger(exam.id) && exam.id > 0
    && ['course_code', 'course_name', 'department', 'date', 'start_time', 'end_time'].every((key) => typeof exam[key] === 'string')
    && /^\d{4}-\d{2}-\d{2}$/.test(exam.date)
    && /^\d{2}:\d{2}:\d{2}$/.test(exam.start_time) && /^\d{2}:\d{2}:\d{2}$/.test(exam.end_time)
    && Number.isInteger(exam.term) && exam.term >= 1 && exam.term <= 8
    && Array.isArray(exam.rooms) && exam.rooms.every((room) => room && typeof room.name === 'string');
}

export async function fetchProfile(token, signal) {
  const response = await fetch('/api/v1/me', {
    headers: { Accept: 'application/json', Authorization: `Bearer ${token}` },
    cache: 'no-store',
    signal,
  });
  let body;
  try { body = await response.json(); } catch {
    throw new Error('Your account information could not be read. Please try again.');
  }
  if (!response.ok) {
    const error = new Error(body?.error?.message || 'Your account could not be loaded.');
    error.status = response.status;
    throw error;
  }
  if (!body || !Number.isSafeInteger(body.id) || body.id < 1 || typeof body.username !== 'string'
    || !['member', 'administrator'].includes(body.role) || !Array.isArray(body.permissions)
    || !body.permissions.every((value) => value === 'invigilator')
    || !(body.personnel_id === null || (Number.isSafeInteger(body.personnel_id) && body.personnel_id > 0))) {
    throw new Error('Your account information is incomplete. Please try again.');
  }
  return body;
}

async function requestResponse(path, token, { method = 'GET', body, signal } = {}, accept = 'application/json') {
  const response = await fetch(`/api/v1/${path}`, {
    method, signal, cache: 'no-store',
    headers: { Accept: accept, Authorization: `Bearer ${token}`, ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  if (!response.ok) {
    let value;
    try { value = await response.json(); } catch { throw new Error('The service returned an unreadable response. Please try again.'); }
    const error = new Error(value?.error?.message || `This action could not be completed (${response.status}).`);
    error.status = response.status;
    error.fields = value?.error?.fields || {};
    throw error;
  }
  return response;
}

export async function requestDownload(path, token, options) {
  const media = path === 'backups' ? 'application/octet-stream' : 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
  const response = await requestResponse(path, token, options, media);
  if (response.headers.get('Content-Type')?.split(';')[0] !== media) throw new Error('The service did not return the requested file. Please try again.');
  return response.blob();
}

export async function requestJSON(path, token, options) {
  const response = await requestResponse(path, token, options);
  if (response.status === 204) return null;
  let value;
  try { value = await response.json(); } catch { throw new Error('The service returned an unreadable response. Please try again.'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('The service returned incomplete information. Please try again.');
  return value;
}

export async function requestResource(path, token, options) {
  const value = await requestJSON(path, token, options);
  if (value === null) return null;
  const record = (entry) => entry && Number.isSafeInteger(entry.id) && entry.id > 0;
  if (!value || (Array.isArray(value.data)
    ? !value.data.every(record) || !Number.isSafeInteger(value.total) || value.total < 0 || !Number.isSafeInteger(value.page) || value.page < 1 || !Number.isSafeInteger(value.page_size) || value.page_size < 1 || value.page_size > 100
    : !record(value))) throw new Error('The service returned incomplete resource information. Please try again.');
  return value;
}
