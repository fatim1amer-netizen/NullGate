import 'storage_service.dart';

enum UserRole { admin, instructor, student }

class User {
  final String name;
  final UserRole role;
  final String? studentCode;
  final String deviceId;
  final String ipAddress;
  final String deviceName;

  User({
    required this.name,
    required this.role,
    this.studentCode,
    this.deviceId = '',
    this.ipAddress = '',
    this.deviceName = '',
  });

  String get roleDisplayName {
    switch (role) {
      case UserRole.admin:
        return 'إداري';
      case UserRole.instructor:
        return 'تدريسي';
      case UserRole.student:
        return 'طالبة';
    }
  }
}

class Project {
  final String id;
  final String title;
  final String description;
  final String supervisor;
  String status;
  final List<ProjectApplication> applications;
  final List<String> acceptedStudents;
  final DateTime createdAt;

  Project({
    required this.id,
    required this.title,
    required this.description,
    required this.supervisor,
    this.status = 'pending',
    List<ProjectApplication>? applications,
    List<String>? acceptedStudents,
    DateTime? createdAt,
  }) : applications = applications ?? [],
       acceptedStudents = acceptedStudents ?? [],
       createdAt = createdAt ?? DateTime.now();

  Map<String, dynamic> toJson() => {
    'id': id,
    'title': title,
    'description': description,
    'supervisor': supervisor,
    'status': status,
    'applications': applications.map((a) => a.toJson()).toList(),
    'acceptedStudents': acceptedStudents,
    'createdAt': createdAt.toIso8601String(),
  };

  factory Project.fromJson(Map<String, dynamic> json) {
    return Project(
      id: json['id'],
      title: json['title'],
      description: json['description'],
      supervisor: json['supervisor'],
      status: json['status'] ?? 'pending',
      applications:
          (json['applications'] as List?)
              ?.map(
                (e) => ProjectApplication.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      acceptedStudents: List<String>.from(json['acceptedStudents'] ?? []),
      createdAt: DateTime.parse(json['createdAt']),
    );
  }
}

class ProjectApplication {
  final String studentName;
  final String fullName;
  final String additionalInfo;
  final String registeredCode;
  final DateTime appliedAt;
  String status;

  ProjectApplication({
    required this.studentName,
    required this.fullName,
    required this.additionalInfo,
    required this.registeredCode,
    DateTime? appliedAt,
    this.status = 'pending',
  }) : appliedAt = appliedAt ?? DateTime.now();

  Map<String, dynamic> toJson() => {
    'studentName': studentName,
    'fullName': fullName,
    'additionalInfo': additionalInfo,
    'registeredCode': registeredCode,
    'appliedAt': appliedAt.toIso8601String(),
    'status': status,
  };

  factory ProjectApplication.fromJson(Map<String, dynamic> json) {
    return ProjectApplication(
      studentName: json['studentName'],
      fullName: json['fullName'],
      additionalInfo: json['additionalInfo'] ?? '',
      registeredCode: json['registeredCode'] ?? '',
      appliedAt: DateTime.parse(json['appliedAt']),
      status: json['status'] ?? 'pending',
    );
  }
}

class Notice {
  final String id;
  final String title;
  final String content;
  final String author;
  final String authorRole;
  final bool pinned;
  final DateTime createdAt;

  Notice({
    required this.id,
    required this.title,
    required this.content,
    required this.author,
    required this.authorRole,
    this.pinned = false,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.now();

  Map<String, dynamic> toJson() => {
    'id': id,
    'title': title,
    'content': content,
    'author': author,
    'authorRole': authorRole,
    'pinned': pinned,
    'createdAt': createdAt.toIso8601String(),
  };

  factory Notice.fromJson(Map<String, dynamic> json) {
    return Notice(
      id: json['id'],
      title: json['title'],
      content: json['content'],
      author: json['author'],
      authorRole: json['authorRole'] ?? '',
      pinned: json['pinned'] ?? false,
      createdAt: DateTime.parse(json['createdAt']),
    );
  }
}

class AppState {
  static User? currentUser;
  static List<Project> projects = [];
  static List<Notice> notices = [];

  static Future<void> loadData() async {
    projects = StorageService.loadProjects();
    notices = StorageService.loadNotices();
  }

  static Future<void> saveProjects() async {
    await StorageService.saveProjects(projects);
  }

  static Future<void> saveNotices() async {
    await StorageService.saveNotices(notices);
  }
}
