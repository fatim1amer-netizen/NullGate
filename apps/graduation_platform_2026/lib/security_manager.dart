import 'storage_service.dart';

enum SecuritySeverity { info, warning, error, critical }

class SecurityLogEntry {
  final String action;
  final String details;
  final String ip;
  final String device;
  final String userName;
  final DateTime timestamp;
  final SecuritySeverity severity;

  SecurityLogEntry({
    required this.action,
    required this.details,
    required this.ip,
    required this.device,
    required this.userName,
    required this.timestamp,
    required this.severity,
  });

  Map<String, dynamic> toJson() => {
    'action': action,
    'details': details,
    'ip': ip,
    'device': device,
    'userName': userName,
    'timestamp': timestamp.toIso8601String(),
    'severity': severity.name,
  };

  factory SecurityLogEntry.fromJson(Map<String, dynamic> json) {
    return SecurityLogEntry(
      action: json['action'] ?? '',
      details: json['details'] ?? '',
      ip: json['ip'] ?? '',
      device: json['device'] ?? '',
      userName: json['userName'] ?? '',
      timestamp: DateTime.parse(json['timestamp']),
      severity: SecuritySeverity.values.firstWhere(
        (e) => e.name == json['severity'],
        orElse: () => SecuritySeverity.info,
      ),
    );
  }
}

class SecurityManager {
  static final List<SecurityLogEntry> securityLogs = [];
  static final Map<String, int> failedAttempts = {};
  static final Map<String, DateTime> lockedAccounts = {};
  static final List<String> bannedAccounts = [];

  static const int maxFailedAttempts = 5;
  static const int lockoutDurationMinutes = 5;

  static Future<void> initialize() async {
    securityLogs.addAll(StorageService.loadSecurityLogs());
    bannedAccounts.addAll(StorageService.loadBannedAccounts());
    await logSecurityEvent(
      action: '🚀 بدء النظام',
      details: 'تم تهيئة مدير الأمان بنجاح',
      ip: 'System',
      device: 'Server',
      userName: 'System',
      severity: SecuritySeverity.info,
    );
  }

  static bool isAccountLocked(String identifier) {
    if (lockedAccounts.containsKey(identifier)) {
      final lockTime = lockedAccounts[identifier]!;
      final timeSinceLock = DateTime.now().difference(lockTime);
      if (timeSinceLock.inMinutes < lockoutDurationMinutes) {
        return true;
      } else {
        lockedAccounts.remove(identifier);
        failedAttempts.remove(identifier);
      }
    }
    return false;
  }

  static bool isAccountBanned(String userName) {
    return bannedAccounts.contains(userName.toLowerCase().trim());
  }

  static Future<void> banAccount(String userName) async {
    final lower = userName.toLowerCase().trim();
    if (!bannedAccounts.contains(lower)) {
      bannedAccounts.add(lower);
      await StorageService.saveBannedAccounts(bannedAccounts);
      await logSecurityEvent(
        action: '🚫 حظر حساب',
        details: 'تم حظر الحساب: $userName',
        ip: 'Admin',
        device: 'Admin Panel',
        userName: 'Admin',
        severity: SecuritySeverity.critical,
      );
    }
  }

  static Future<void> unbanAccount(String userName) async {
    final lower = userName.toLowerCase().trim();
    bannedAccounts.remove(lower);
    await StorageService.saveBannedAccounts(bannedAccounts);
    await logSecurityEvent(
      action: '✅ إلغاء حظر',
      details: 'تم إلغاء حظر الحساب: $userName',
      ip: 'Admin',
      device: 'Admin Panel',
      userName: 'Admin',
      severity: SecuritySeverity.info,
    );
  }

  static Future<void> recordFailedAttempt(
    String identifier,
    String reason,
    String userName,
  ) async {
    failedAttempts[identifier] = (failedAttempts[identifier] ?? 0) + 1;
    final attempts = failedAttempts[identifier]!;

    await logSecurityEvent(
      action: '❌ محاولة فاشلة #$attempts',
      details: reason,
      ip: identifier,
      device: 'Web Browser',
      userName: userName,
      severity: SecuritySeverity.warning,
    );

    if (attempts >= maxFailedAttempts) {
      lockedAccounts[identifier] = DateTime.now();
      await logSecurityEvent(
        action: '🔒 قفل حساب',
        details: 'تم قفل الحساب بعد $attempts محاولات فاشلة',
        ip: identifier,
        device: 'Web Browser',
        userName: userName,
        severity: SecuritySeverity.critical,
      );
    }
  }

  static Future<void> logSecurityEvent({
    required String action,
    required String details,
    required String ip,
    required String device,
    required String userName,
    required SecuritySeverity severity,
  }) async {
    final log = SecurityLogEntry(
      action: action,
      details: details,
      ip: ip,
      device: device,
      userName: userName,
      timestamp: DateTime.now(),
      severity: severity,
    );
    securityLogs.add(log);
    await StorageService.saveSecurityLogs(securityLogs);
  }

  static int? getRemainingLockTime(String identifier) {
    if (!lockedAccounts.containsKey(identifier)) return null;
    final lockTime = lockedAccounts[identifier]!;
    final elapsed = DateTime.now().difference(lockTime).inMinutes;
    final remaining = lockoutDurationMinutes - elapsed;
    return remaining > 0 ? remaining : null;
  }

  static Map<String, dynamic> getSecurityStats() {
    return {
      'totalLogs': securityLogs.length,
      'criticalEvents': securityLogs
          .where((l) => l.severity == SecuritySeverity.critical)
          .length,
      'warningEvents': securityLogs
          .where((l) => l.severity == SecuritySeverity.warning)
          .length,
      'lockedAccounts': lockedAccounts.length,
      'bannedAccounts': bannedAccounts.length,
    };
  }
}
