const department = { name: 'department_id', label: 'Department', type: 'number', reference: 'departments', required: true };
const session = { name: 'session_id', label: 'Session', type: 'number', reference: 'sessions', required: true };
const personnel = { name: 'personnel_id', label: 'Personnel', type: 'number', reference: 'personnel', required: true };
const dateRange = [{ name: 'date_from', label: 'From date', type: 'date' }, { name: 'date_to', label: 'To date', type: 'date' }];

export const resources = {
  departments: {
    title: 'Departments', singular: 'department', icon: 'department', description: 'Organize the faculty departments used by courses and personnel.',
    fields: [{ name: 'name', label: 'Name', required: true, maxLength: 100 }],
    filters: [{ name: 'name', label: 'Department name' }],
    heading: (value) => value.name,
  },
  courses: {
    title: 'Courses', singular: 'course', icon: 'book', description: 'Manage enrollment and the department and term for each course.',
    fields: [
      { name: 'code', label: 'Code', required: true, maxLength: 20 }, { name: 'name', label: 'Name', required: true, maxLength: 100 },
      { name: 'kind', label: 'Kind', options: ['mandatory', 'elective'], required: true },
      { name: 'enrollment', label: 'Enrollment', type: 'number', required: true },
      { name: 'term', label: 'Term', type: 'number', min: 1, max: 8, required: true }, department,
    ],
    filters: [{ name: 'q', label: 'Course name or code' }, { name: 'department_id', label: 'Department ID', type: 'number' }, { name: 'term', label: 'Term', type: 'number', min: 1, max: 8 }, { name: 'kind', label: 'Kind', options: ['mandatory', 'elective'] }],
    heading: (value) => `${value.code} · ${value.name}`,
  },
  classrooms: {
    title: 'Classrooms', singular: 'classroom', icon: 'room', description: 'Set room capacity, floor, and availability for exam allocation.',
    fields: [
      { name: 'name', label: 'Name', required: true, maxLength: 50 }, { name: 'capacity', label: 'Capacity', type: 'number', required: true },
      { name: 'type', label: 'Room type', required: true, maxLength: 50 }, { name: 'floor', label: 'Floor', type: 'number', min: -2147483648, required: true, initial: 0 },
      { name: 'active', label: 'Available for allocation', type: 'checkbox', initial: true },
    ],
    filters: [{ name: 'active', label: 'Available', options: ['true', 'false'] }, { name: 'floor', label: 'Floor', type: 'number', min: -2147483648 }, { name: 'min_capacity', label: 'Minimum capacity', type: 'number' }],
    heading: (value) => value.name,
  },
  sessions: {
    title: 'Sessions', singular: 'session', icon: 'clock', description: 'Define non-overlapping exam times in Europe/Istanbul.',
    fields: [{ name: 'date', label: 'Date', type: 'date', required: true }, { name: 'start_time', label: 'Start time', type: 'time', required: true }, { name: 'end_time', label: 'End time', type: 'time', required: true }, { name: 'description', label: 'Description', type: 'textarea', maxLength: 500 }],
    filters: dateRange,
    heading: (value) => `${value.date} · ${value.start_time.slice(0, 5)}–${value.end_time.slice(0, 5)}`,
  },
  personnel: {
    title: 'Personnel', singular: 'personnel record', icon: 'account', description: 'Manage faculty personnel and their departments.',
    fields: [{ name: 'title', label: 'Title', maxLength: 100 }, { name: 'first_name', label: 'First name', required: true, maxLength: 100 }, { name: 'last_name', label: 'Last name', required: true, maxLength: 100 }, department],
    filters: [{ name: 'q', label: 'Personnel name' }, { name: 'department_id', label: 'Department ID', type: 'number' }],
    heading: (value) => `${value.title} ${value.first_name} ${value.last_name}`.trim(),
  },
  'unavailability-requests': {
    title: 'Unavailability requests', singular: 'request', icon: 'leave', description: 'Manage pending leave requests and review their decisions.',
    fields: [
      { ...personnel, adminOnly: true, immutable: true }, { name: 'date', label: 'Date', type: 'date', required: true },
      { ...session, required: false, help: 'Optional. A selected session determines the request date.' },
      { name: 'type', label: 'Request type', options: ['leave', 'medical', 'advisory'], required: true },
      { name: 'reason', label: 'Reason', type: 'textarea', maxLength: 500, required: true },
    ],
    filters: [{ name: 'personnel_id', label: 'Personnel ID', type: 'number', adminOnly: true }, ...dateRange, { name: 'status', label: 'Status', options: ['pending', 'approved', 'rejected'] }, { name: 'type', label: 'Request type', options: ['leave', 'medical', 'advisory'] }],
    heading: (value) => `${value.date} · ${value.type}`,
  },
  exams: {
    title: 'Exams', singular: 'exam', icon: 'calendar', description: 'Schedule a course in a session. Rooms are allocated automatically; room changes and duties are validated together.',
    fields: [{ name: 'course_id', label: 'Course', type: 'number', reference: 'courses', required: true }, session],
    filters: [{ name: 'course_id', label: 'Course ID', type: 'number' }, { name: 'department_id', label: 'Department ID', type: 'number' }, { name: 'term', label: 'Term', type: 'number', min: 1, max: 8 }, { name: 'session_id', label: 'Session ID', type: 'number' }, ...dateRange],
    heading: (value) => `${value.course_code} · ${value.course_name}`,
  },
  'invigilator-assignments': {
    title: 'Invigilator assignments', singular: 'assignment', icon: 'check', description: 'Assign one available invigilator to an allocated exam room. Availability and duty limits are checked on every change.',
    fields: [{ name: 'id', label: 'Exam room allocation', type: 'number', reference: 'allocations', required: true, createOnly: true }, personnel],
    filters: [{ name: 'personnel_id', label: 'Personnel ID', type: 'number' }, { name: 'exam_id', label: 'Exam ID', type: 'number' }, { name: 'session_id', label: 'Session ID', type: 'number' }],
    heading: (value) => `${value.course_code} · ${value.classroom}`,
  },
};

export function resourceFields(resource, administrator, editing) {
  return resources[resource].fields.filter((field) => (!field.adminOnly || administrator) && (!field.createOnly || !editing));
}

export function resourceBody(fields, values) {
  return Object.fromEntries(fields.map((field) => [field.name, field.type === 'checkbox' ? Boolean(values[field.name]) : field.type === 'number' ? values[field.name] === '' || values[field.name] == null ? null : Number(values[field.name]) : String(values[field.name] ?? '').trim()]));
}
