import 'package:flutter/material.dart';
import 'app_theme.dart';
import 'security_manager.dart';
import 'data_models.dart';
import 'glass_widgets.dart';
import 'animated_background.dart';
import 'responsive_layout.dart';

// ============== Publish Project ==============
class PublishProjectPage extends StatefulWidget {
  const PublishProjectPage({super.key});

  @override
  State<PublishProjectPage> createState() => _PublishProjectPageState();
}

class _PublishProjectPageState extends State<PublishProjectPage> {
  final _titleController = TextEditingController();
  final _descriptionController = TextEditingController();

  Future<void> _publish() async {
    final title = _titleController.text.trim();
    final description = _descriptionController.text.trim();

    if (title.isEmpty || description.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('يرجى ملء جميع الحقول'),
          backgroundColor: AppColors.danger,
        ),
      );
      return;
    }

    final project = Project(
      id: DateTime.now().millisecondsSinceEpoch.toString(),
      title: title,
      description: description,
      supervisor: AppState.currentUser!.name,
    );

    AppState.projects.add(project);
    await AppState.saveProjects();

    await SecurityManager.logSecurityEvent(
      action: '📝 نشر مشروع',
      details: title,
      ip: AppState.currentUser!.ipAddress,
      device: AppState.currentUser!.deviceName,
      userName: AppState.currentUser!.name,
      severity: SecuritySeverity.info,
    );

    if (mounted) {
      Navigator.pop(context);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('تم نشر المشروع'),
          backgroundColor: AppColors.success,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return AuroraBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: AppBar(title: const Text('نشر مشروع جديد')),
        body: SingleChildScrollView(
          padding: ResponsiveUtils.responsivePadding(context),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 700),
            child: NeonGlassCard(
              margin: EdgeInsets.zero,
              padding: const EdgeInsets.all(24),
              glowColor: AppColors.primaryPink,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const SectionHeader(
                    title: 'معلومات المشروع',
                    subtitle: 'Project Details',
                    icon: Icons.add_circle,
                    iconGradient: AppColors.auroraGradient,
                  ),
                  const SizedBox(height: 24),
                  PremiumTextField(
                    controller: _titleController,
                    label: 'عنوان المشروع',
                    hint: 'أدخل عنوان المشروع',
                    icon: Icons.title,
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    controller: _descriptionController,
                    maxLines: 8,
                    style: Theme.of(
                      context,
                    ).textTheme.bodyLarge?.copyWith(color: AppColors.whiteText),
                    decoration: const InputDecoration(
                      labelText: 'وصف المشروع',
                      hintText: 'أدخل وصف تفصيلي...',
                      prefixIcon: Icon(Icons.description),
                      alignLabelWithHint: true,
                    ),
                  ),
                  const SizedBox(height: 24),
                  PremiumButton(
                    text: 'نشر المشروع',
                    icon: Icons.publish_rounded,
                    onPressed: _publish,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
    super.dispose();
  }
}

// ============== Project Detail ==============
class ProjectDetailPage extends StatefulWidget {
  final Project project;
  const ProjectDetailPage({super.key, required this.project});

  @override
  State<ProjectDetailPage> createState() => _ProjectDetailPageState();
}

class _ProjectDetailPageState extends State<ProjectDetailPage> {
  final _fullNameController = TextEditingController();
  final _additionalInfoController = TextEditingController();

  @override
  Widget build(BuildContext context) {
    final user = AppState.currentUser!;
    final isSupervisor = widget.project.supervisor == user.name;

    ProjectApplication? existingApplication;
    for (var app in widget.project.applications) {
      if (app.studentName == user.name) {
        existingApplication = app;
        break;
      }
    }

    final hasApplied = existingApplication != null;
    final isAccepted = widget.project.acceptedStudents.contains(user.name);

    return AuroraBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: AppBar(
          title: Text(widget.project.title),
          actions: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              margin: const EdgeInsets.only(left: 16),
              decoration: BoxDecoration(
                gradient: AppColors.pinkCyanGradient,
                borderRadius: BorderRadius.circular(20),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.school, color: Colors.white, size: 14),
                  const SizedBox(width: 6),
                  Text(
                    user.studentCode ?? user.name,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Colors.white,
                      fontWeight: FontWeight.w600,
                      fontSize: 11,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
        body: SingleChildScrollView(
          padding: ResponsiveUtils.responsivePadding(context),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 900),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                NeonGlassCard(
                  margin: EdgeInsets.zero,
                  padding: const EdgeInsets.all(24),
                  glowColor: AppColors.primaryPink,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              gradient: AppColors.auroraGradient,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: const Icon(
                              Icons.folder,
                              color: Colors.white,
                              size: 24,
                            ),
                          ),
                          const SizedBox(width: 16),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  widget.project.title,
                                  style: Theme.of(
                                    context,
                                  ).textTheme.headlineMedium,
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  'بواسطة ${widget.project.supervisor}',
                                  style: Theme.of(context).textTheme.bodySmall
                                      ?.copyWith(color: AppColors.primaryPink),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 16),
                      const Divider(),
                      const SizedBox(height: 16),
                      Text(
                        widget.project.description,
                        style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: AppColors.secondaryText,
                          height: 1.7,
                        ),
                      ),
                      const SizedBox(height: 16),
                      Wrap(
                        spacing: 12,
                        runSpacing: 8,
                        children: [
                          PremiumBadge(
                            text: 'المشرف: ${widget.project.supervisor}',
                            color: AppColors.electricCyan,
                            icon: Icons.person,
                          ),
                          PremiumBadge(
                            text: '${widget.project.applications.length} طلب',
                            color: AppColors.primaryPink,
                            icon: Icons.assignment,
                          ),
                          PremiumBadge(
                            text:
                                '${widget.project.acceptedStudents.length} مقبول',
                            color: AppColors.success,
                            icon: Icons.check_circle,
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 16),

                if (user.role == UserRole.student && !isSupervisor) ...[
                  if (!hasApplied)
                    NeonGlassCard(
                      margin: EdgeInsets.zero,
                      padding: const EdgeInsets.all(24),
                      glowColor: AppColors.electricCyan,
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const SectionHeader(
                            title: 'تقديم طلب انضمام',
                            subtitle: 'Join Request',
                            icon: Icons.send,
                            iconGradient: AppColors.pinkCyanGradient,
                          ),
                          const SizedBox(height: 20),
                          PremiumTextField(
                            controller: _fullNameController,
                            label: 'الاسم الكامل',
                            hint: 'اكتب اسمك الكامل كما تريد عرضه',
                            icon: Icons.person_outline,
                          ),
                          const SizedBox(height: 16),
                          TextField(
                            controller: _additionalInfoController,
                            maxLines: 4,
                            style: Theme.of(context).textTheme.bodyLarge
                                ?.copyWith(color: AppColors.whiteText),
                            decoration: const InputDecoration(
                              labelText: 'الإضافات والمعلومات الإضافية',
                              hintText: 'اكتب أي معلومات إضافية...',
                              prefixIcon: Icon(Icons.notes),
                              alignLabelWithHint: true,
                            ),
                          ),
                          const SizedBox(height: 20),
                          PremiumButton(
                            text: 'إرسال الطلب',
                            icon: Icons.send_rounded,
                            onPressed: _submitApplication,
                          ),
                        ],
                      ),
                    )
                  else if (isAccepted)
                    NeonGlassCard(
                      margin: EdgeInsets.zero,
                      padding: const EdgeInsets.all(24),
                      glowColor: AppColors.success,
                      child: Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              gradient: AppColors.successGradient,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: const Icon(
                              Icons.check_circle,
                              color: Colors.white,
                              size: 28,
                            ),
                          ),
                          const SizedBox(width: 16),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  'تم قبولك في المشروع!',
                                  style: Theme.of(context).textTheme.titleMedium
                                      ?.copyWith(
                                        color: AppColors.success,
                                        fontWeight: FontWeight.bold,
                                      ),
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  'تهانينا، تم قبول طلبك بنجاح',
                                  style: Theme.of(context).textTheme.bodySmall,
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    )
                  else
                    NeonGlassCard(
                      margin: EdgeInsets.zero,
                      padding: const EdgeInsets.all(24),
                      glowColor: AppColors.warning,
                      child: Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              color: AppColors.warning.withOpacity(0.15),
                              borderRadius: BorderRadius.circular(12),
                              border: Border.all(
                                color: AppColors.warning.withOpacity(0.3),
                              ),
                            ),
                            child: const Icon(
                              Icons.hourglass_empty,
                              color: AppColors.warning,
                              size: 28,
                            ),
                          ),
                          const SizedBox(width: 16),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  'طلبك قيد المراجعة',
                                  style: Theme.of(context).textTheme.titleMedium
                                      ?.copyWith(color: AppColors.warning),
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  'سيتم مراجعة طلبك من قبل المشرف',
                                  style: Theme.of(context).textTheme.bodySmall,
                                ),
                                const SizedBox(height: 8),
                                Text(
                                  'الحالة: ${_getStatusText(existingApplication.status)}',
                                  style: Theme.of(context).textTheme.bodySmall
                                      ?.copyWith(
                                        color: _getStatusColor(
                                          existingApplication.status,
                                        ),
                                        fontWeight: FontWeight.w600,
                                      ),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                ],

                if (isSupervisor && widget.project.applications.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  const SectionHeader(
                    title: 'الطلبات المقدمة',
                    subtitle: 'Applications',
                    icon: Icons.assignment,
                    iconGradient: AppColors.auroraGradient,
                  ),
                  const SizedBox(height: 12),
                  ...widget.project.applications.map((app) {
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 12),
                      child: NeonGlassCard(
                        margin: EdgeInsets.zero,
                        padding: const EdgeInsets.all(20),
                        glowColor: AppColors.primaryPink,
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              children: [
                                Container(
                                  padding: const EdgeInsets.all(10),
                                  decoration: BoxDecoration(
                                    gradient: AppColors.auroraGradient,
                                    borderRadius: BorderRadius.circular(12),
                                  ),
                                  child: const Icon(
                                    Icons.person,
                                    color: Colors.white,
                                    size: 20,
                                  ),
                                ),
                                const SizedBox(width: 12),
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        'الاسم المسجل: ${app.studentName}',
                                        style: Theme.of(context)
                                            .textTheme
                                            .titleMedium
                                            ?.copyWith(
                                              fontWeight: FontWeight.bold,
                                            ),
                                      ),
                                      const SizedBox(height: 4),
                                      Text(
                                        'الاسم الكامل: ${app.fullName}',
                                        style: Theme.of(context)
                                            .textTheme
                                            .bodySmall
                                            ?.copyWith(
                                              color: AppColors.electricCyan,
                                            ),
                                      ),
                                      Text(
                                        'الكود: ${app.registeredCode}',
                                        style: Theme.of(
                                          context,
                                        ).textTheme.bodySmall,
                                      ),
                                    ],
                                  ),
                                ),
                                PremiumBadge(
                                  text: _getStatusText(app.status),
                                  color: _getStatusColor(app.status),
                                ),
                              ],
                            ),
                            if (app.additionalInfo.isNotEmpty) ...[
                              const SizedBox(height: 12),
                              Container(
                                padding: const EdgeInsets.all(12),
                                decoration: BoxDecoration(
                                  color: AppColors.midnightBlue,
                                  borderRadius: BorderRadius.circular(12),
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      'الإضافات:',
                                      style: Theme.of(context)
                                          .textTheme
                                          .bodySmall
                                          ?.copyWith(
                                            color: AppColors.primaryPink,
                                            fontWeight: FontWeight.bold,
                                          ),
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      app.additionalInfo,
                                      style: Theme.of(
                                        context,
                                      ).textTheme.bodyMedium,
                                    ),
                                  ],
                                ),
                              ),
                            ],
                            if (app.status == 'pending') ...[
                              const SizedBox(height: 16),
                              Row(
                                children: [
                                  Expanded(
                                    child: OutlinedButton.icon(
                                      onPressed: () => _acceptApplication(app),
                                      icon: const Icon(Icons.check, size: 18),
                                      label: const Text('قبول'),
                                      style: OutlinedButton.styleFrom(
                                        foregroundColor: AppColors.success,
                                        side: const BorderSide(
                                          color: AppColors.success,
                                        ),
                                        padding: const EdgeInsets.symmetric(
                                          vertical: 12,
                                        ),
                                      ),
                                    ),
                                  ),
                                  const SizedBox(width: 8),
                                  Expanded(
                                    child: OutlinedButton.icon(
                                      onPressed: () => _rejectApplication(app),
                                      icon: const Icon(Icons.close, size: 18),
                                      label: const Text('رفض'),
                                      style: OutlinedButton.styleFrom(
                                        foregroundColor: AppColors.danger,
                                        side: const BorderSide(
                                          color: AppColors.danger,
                                        ),
                                        padding: const EdgeInsets.symmetric(
                                          vertical: 12,
                                        ),
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          ],
                        ),
                      ),
                    );
                  }),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  Color _getStatusColor(String status) {
    switch (status) {
      case 'accepted':
        return AppColors.success;
      case 'rejected':
        return AppColors.danger;
      default:
        return AppColors.warning;
    }
  }

  String _getStatusText(String status) {
    switch (status) {
      case 'accepted':
        return 'مقبول';
      case 'rejected':
        return 'مرفوض';
      default:
        return 'قيد المراجعة';
    }
  }

  Future<void> _submitApplication() async {
    final fullName = _fullNameController.text.trim();
    final additionalInfo = _additionalInfoController.text.trim();
    final user = AppState.currentUser!;

    if (fullName.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('يرجى كتابة الاسم الكامل'),
          backgroundColor: AppColors.danger,
        ),
      );
      return;
    }

    final application = ProjectApplication(
      studentName: user.name,
      fullName: fullName,
      additionalInfo: additionalInfo,
      registeredCode: user.studentCode ?? '',
    );

    setState(() => widget.project.applications.add(application));
    await AppState.saveProjects();

    await SecurityManager.logSecurityEvent(
      action: '📩 تقديم طلب',
      details: '${user.name} تقدم لـ ${widget.project.title}',
      ip: user.ipAddress,
      device: user.deviceName,
      userName: user.name,
      severity: SecuritySeverity.info,
    );

    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('تم إرسال الطلب بنجاح'),
        backgroundColor: AppColors.success,
      ),
    );
  }

  Future<void> _acceptApplication(ProjectApplication app) async {
    setState(() {
      app.status = 'accepted';
      if (!widget.project.acceptedStudents.contains(app.studentName)) {
        widget.project.acceptedStudents.add(app.studentName);
      }
    });
    await AppState.saveProjects();

    await SecurityManager.logSecurityEvent(
      action: '✅ قبول طالب',
      details: '${app.fullName} مقبول في ${widget.project.title}',
      ip: AppState.currentUser!.ipAddress,
      device: AppState.currentUser!.deviceName,
      userName: AppState.currentUser!.name,
      severity: SecuritySeverity.info,
    );

    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('تم قبول ${app.fullName}'),
        backgroundColor: AppColors.success,
      ),
    );
  }

  Future<void> _rejectApplication(ProjectApplication app) async {
    setState(() => app.status = 'rejected');
    await AppState.saveProjects();

    await SecurityManager.logSecurityEvent(
      action: '❌ رفض طالب',
      details: '${app.fullName} مرفوض من ${widget.project.title}',
      ip: AppState.currentUser!.ipAddress,
      device: AppState.currentUser!.deviceName,
      userName: AppState.currentUser!.name,
      severity: SecuritySeverity.warning,
    );

    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('تم رفض ${app.fullName}'),
        backgroundColor: AppColors.warning,
      ),
    );
  }

  @override
  void dispose() {
    _fullNameController.dispose();
    _additionalInfoController.dispose();
    super.dispose();
  }
}

// ============== Publish Notice ==============
class PublishNoticePage extends StatefulWidget {
  const PublishNoticePage({super.key});

  @override
  State<PublishNoticePage> createState() => _PublishNoticePageState();
}

class _PublishNoticePageState extends State<PublishNoticePage> {
  final _titleController = TextEditingController();
  final _contentController = TextEditingController();
  bool _pinned = false;

  Future<void> _publish() async {
    final title = _titleController.text.trim();
    final content = _contentController.text.trim();

    if (title.isEmpty || content.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('يرجى ملء جميع الحقول'),
          backgroundColor: AppColors.danger,
        ),
      );
      return;
    }

    final user = AppState.currentUser!;
    final notice = Notice(
      id: DateTime.now().millisecondsSinceEpoch.toString(),
      title: title,
      content: content,
      author: user.name,
      authorRole: user.roleDisplayName,
      pinned: _pinned,
    );

    AppState.notices.add(notice);
    await AppState.saveNotices();

    await SecurityManager.logSecurityEvent(
      action: '📢 نشر تبليغ',
      details: title,
      ip: user.ipAddress,
      device: user.deviceName,
      userName: user.name,
      severity: SecuritySeverity.info,
    );

    if (mounted) {
      Navigator.pop(context);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('تم نشر التبليغ'),
          backgroundColor: AppColors.success,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return AuroraBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: AppBar(title: const Text('نشر تبليغ جديد')),
        body: SingleChildScrollView(
          padding: ResponsiveUtils.responsivePadding(context),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 700),
            child: NeonGlassCard(
              margin: EdgeInsets.zero,
              padding: const EdgeInsets.all(24),
              glowColor: AppColors.electricCyan,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const SectionHeader(
                    title: 'معلومات التبليغ',
                    subtitle: 'Notice Details',
                    icon: Icons.campaign,
                    iconGradient: AppColors.pinkCyanGradient,
                  ),
                  const SizedBox(height: 24),
                  PremiumTextField(
                    controller: _titleController,
                    label: 'عنوان التبليغ',
                    hint: 'أدخل عنوان التبليغ',
                    icon: Icons.title,
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    controller: _contentController,
                    maxLines: 8,
                    style: Theme.of(
                      context,
                    ).textTheme.bodyLarge?.copyWith(color: AppColors.whiteText),
                    decoration: const InputDecoration(
                      labelText: 'محتوى التبليغ',
                      hintText: 'أدخل المحتوى...',
                      prefixIcon: Icon(Icons.article),
                      alignLabelWithHint: true,
                    ),
                  ),
                  const SizedBox(height: 16),
                  Row(
                    children: [
                      const Icon(
                        Icons.push_pin,
                        color: AppColors.primaryPink,
                        size: 20,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        'تثبيت التبليغ',
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                      const Spacer(),
                      Switch(
                        value: _pinned,
                        onChanged: (value) => setState(() => _pinned = value),
                      ),
                    ],
                  ),
                  const SizedBox(height: 24),
                  PremiumButton(
                    text: 'نشر التبليغ',
                    icon: Icons.publish_rounded,
                    onPressed: _publish,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  @override
  void dispose() {
    _titleController.dispose();
    _contentController.dispose();
    super.dispose();
  }
}
