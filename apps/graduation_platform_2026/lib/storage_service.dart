import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';
import 'data_models.dart';
import 'security_manager.dart';

class StorageService {
  static late SharedPreferences _prefs;

  static Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
  }

  static Future<void> saveProjects(List<Project> projects) async {
    final List<Map<String, dynamic>> data = projects
        .map((p) => p.toJson())
        .toList();
    await _prefs.setString('projects', jsonEncode(data));
  }

  static List<Project> loadProjects() {
    final String? data = _prefs.getString('projects');
    if (data == null) return [];
    try {
      final List<dynamic> list = jsonDecode(data);
      return list
          .map((e) => Project.fromJson(e as Map<String, dynamic>))
          .toList();
    } catch (e) {
      return [];
    }
  }

  static Future<void> saveNotices(List<Notice> notices) async {
    final List<Map<String, dynamic>> data = notices
        .map((n) => n.toJson())
        .toList();
    await _prefs.setString('notices', jsonEncode(data));
  }

  static List<Notice> loadNotices() {
    final String? data = _prefs.getString('notices');
    if (data == null) return [];
    try {
      final List<dynamic> list = jsonDecode(data);
      return list
          .map((e) => Notice.fromJson(e as Map<String, dynamic>))
          .toList();
    } catch (e) {
      return [];
    }
  }

  static Future<void> saveBannedAccounts(List<String> banned) async {
    await _prefs.setStringList('banned_accounts', banned);
  }

  static List<String> loadBannedAccounts() {
    return _prefs.getStringList('banned_accounts') ?? [];
  }

  static Future<void> saveSecurityLogs(List<SecurityLogEntry> logs) async {
    final List<Map<String, dynamic>> data = logs
        .map((l) => l.toJson())
        .toList();
    await _prefs.setString('security_logs', jsonEncode(data));
  }

  static List<SecurityLogEntry> loadSecurityLogs() {
    final String? data = _prefs.getString('security_logs');
    if (data == null) return [];
    try {
      final List<dynamic> list = jsonDecode(data);
      return list
          .map((e) => SecurityLogEntry.fromJson(e as Map<String, dynamic>))
          .toList();
    } catch (e) {
      return [];
    }
  }

  static Future<void> clearAll() async {
    await _prefs.clear();
  }
}
