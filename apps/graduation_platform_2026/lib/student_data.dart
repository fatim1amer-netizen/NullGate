class StudentData {
  static final Map<String, String> students = {
    'DEMO01': 'طالبة تجريبية 01',
    'DEMO02': 'طالبة تجريبية 02',
    'DEMO03': 'طالبة تجريبية 03',
    'DEMO04': 'طالبة تجريبية 04',
    'DEMO05': 'طالبة تجريبية 05',
    'DEMO06': 'طالبة تجريبية 06',
    'DEMO07': 'طالبة تجريبية 07',
    'DEMO08': 'طالبة تجريبية 08',
    'DEMO09': 'طالبة تجريبية 09',
    'DEMO10': 'طالبة تجريبية 10',
    'DEMO11': 'طالبة تجريبية 11',
    'DEMO12': 'طالبة تجريبية 12',
    'DEMO13': 'طالبة تجريبية 13',
    'DEMO14': 'طالبة تجريبية 14',
    'DEMO15': 'طالبة تجريبية 15',
    'DEMO16': 'طالبة تجريبية 16',
    'DEMO17': 'طالبة تجريبية 17',
    'DEMO18': 'طالبة تجريبية 18',
    'DEMO19': 'طالبة تجريبية 19',
    'DEMO20': 'طالبة تجريبية 20',
    'DEMO21': 'طالبة تجريبية 21',
    'DEMO22': 'طالبة تجريبية 22',
    'DEMO23': 'طالبة تجريبية 23',
    'DEMO24': 'طالبة تجريبية 24',
    'DEMO25': 'طالبة تجريبية 25',
    'DEMO26': 'طالبة تجريبية 26',
    'DEMO27': 'طالبة تجريبية 27',
    'DEMO28': 'طالبة تجريبية 28',
    'DEMO29': 'طالبة تجريبية 29',
    'DEMO30': 'طالبة تجريبية 30',
    'DEMO31': 'طالبة تجريبية 31',
  };

  static String? findStudentByName(String name) {
    final normalized = normalizeName(name);
    for (var entry in students.entries) {
      if (normalizeName(entry.value) == normalized) {
        return entry.key;
      }
    }
    return null;
  }

  static String? findStudentByCode(String code) {
    return students[code.toUpperCase().trim()];
  }

  static String normalizeName(String name) {
    return name
        .replaceAll(RegExp(r'[\u064B-\u065F]'), '')
        .replaceAll(RegExp(r'\s+'), ' ')
        .trim();
  }

  static bool validateCredentials(String name, String code) {
    final studentName = findStudentByCode(code);
    if (studentName == null) return false;
    return normalizeName(studentName) == normalizeName(name);
  }

  static int get studentCount => students.length;
}
