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
