import 'package:flutter/material.dart';
import 'app_theme.dart';
import 'security_manager.dart';
import 'data_models.dart';
import 'student_data.dart';
import 'animated_background.dart';
import 'glass_widgets.dart';
import 'responsive_layout.dart';
import 'pages.dart';

// ============================================================================
// 🎬 CINEMATIC SPLASH SCREEN
// ============================================================================
class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen>
    with TickerProviderStateMixin {
  late AnimationController _logoController;
  late AnimationController _textController;
  late Animation<double> _logoScale;
  late Animation<double> _logoFade;
  late Animation<Offset> _titleSlide;
  late Animation<double> _titleFade;
  late Animation<double> _subtitleFade;
  late Animation<double> _dividerWidth;

  @override
  void initState() {
    super.initState();

    _logoController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1500),
    );
    _logoScale = Tween<double>(begin: 0.5, end: 1.0).animate(
      CurvedAnimation(
        parent: _logoController,
        curve: const Interval(0.0, 0.6, curve: Curves.elasticOut),
      ),
    );
    _logoFade = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(
        parent: _logoController,
        curve: const Interval(0.0, 0.4, curve: Curves.easeOut),
      ),
    );

    _textController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    );
    _titleSlide = Tween<Offset>(begin: const Offset(0, 0.3), end: Offset.zero)
        .animate(
          CurvedAnimation(
            parent: _textController,
            curve: const Interval(0.2, 0.7, curve: Curves.easeOutCubic),
          ),
        );
    _titleFade = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(
        parent: _textController,
        curve: const Interval(0.2, 0.6, curve: Curves.easeOut),
      ),
    );
    _subtitleFade = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(
        parent: _textController,
        curve: const Interval(0.5, 0.9, curve: Curves.easeOut),
      ),
    );
    _dividerWidth = Tween<double>(begin: 0.0, end: 100.0).animate(
      CurvedAnimation(
        parent: _textController,
        curve: const Interval(0.4, 0.8, curve: Curves.easeOutCubic),
      ),
    );

    _logoController.forward();
    Future.delayed(const Duration(milliseconds: 400), () {
      if (mounted) _textController.forward();
    });

    Future.delayed(const Duration(seconds: 5), () {
      if (mounted) {
        Navigator.pushReplacement(
          context,
          PageRouteBuilder(
            pageBuilder: (context, animation, secondaryAnimation) =>
                const WelcomeScreen(),
            transitionsBuilder:
                (context, animation, secondaryAnimation, child) {
                  return FadeTransition(opacity: animation, child: child);
                },
            transitionDuration: const Duration(milliseconds: 800),
          ),
        );
      }
    });
  }

  @override
  void dispose() {
    _logoController.dispose();
    _textController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: AuroraBackground(
        child: Center(
          child: Padding(
            padding: ResponsiveUtils.responsivePadding(context),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                FadeTransition(
                  opacity: _logoFade,
                  child: ScaleTransition(
                    scale: _logoScale,
                    child: PremiumLogo(size: 160),
                  ),
                ),
                const SizedBox(height: 40),
                SlideTransition(
                  position: _titleSlide,
                  child: FadeTransition(
                    opacity: _titleFade,
                    child: Column(
                      children: [
                        Text(
                          'منصة إدارة مشاريع التخرج',
                          style: Theme.of(context).textTheme.displayMedium
                              ?.copyWith(color: Colors.white),
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: 12),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 16,
                            vertical: 6,
                          ),
                          decoration: BoxDecoration(
                            gradient: AppColors.auroraGradient,
                            borderRadius: BorderRadius.circular(20),
                            boxShadow: [
                              BoxShadow(
                                color: AppColors.primaryPink.withOpacity(0.4),
                                blurRadius: 15,
                              ),
                            ],
                          ),
                          child: Text(
                            'GRADUATION PLATFORM 2026',
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(
                                  color: Colors.white,
                                  letterSpacing: 3,
                                  fontWeight: FontWeight.w600,
                                  fontSize: 11,
                                ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 32),
                FadeTransition(
                  opacity: _subtitleFade,
                  child: Column(
                    children: [
                      AnimatedBuilder(
                        animation: _textController,
                        builder: (context, child) {
                          return Container(
                            width: _dividerWidth.value,
                            height: 1,
                            decoration: const BoxDecoration(
                              gradient: AppColors.auroraGradient,
                            ),
                          );
                        },
                      ),
                      const SizedBox(height: 24),
                      _buildInfoLine(
                        Icons.business,
                        'جامعة الزهراء',
                        AppColors.electricCyan,
                      ),
                      const SizedBox(height: 10),
                      _buildInfoLine(
                        Icons.engineering,
                        'كلية هندسة تكنولوجيا المعلومات',
                        AppColors.secondaryText,
                      ),
                      const SizedBox(height: 10),
                      _buildInfoLine(
                        Icons.security,
                        'قسم الأمن السيبراني',
                        AppColors.primaryPink,
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildInfoLine(IconData icon, String text, Color color) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        BreathingOrb(size: 8, color: color),
        const SizedBox(width: 10),
        Text(
          text,
          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
            color: color,
            fontWeight: FontWeight.w500,
          ),
        ),
      ],
    );
  }
}

// ============================================================================
// 🌟 PREMIUM WELCOME SCREEN
// ============================================================================
class WelcomeScreen extends StatefulWidget {
  const WelcomeScreen({super.key});

  @override
  State<WelcomeScreen> createState() => _WelcomeScreenState();
}

class _WelcomeScreenState extends State<WelcomeScreen>
    with TickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _fadeIn;
  late Animation<Offset> _slideUp;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1500),
    );
    _fadeIn = Tween<double>(
      begin: 0.0,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeOut));
    _slideUp = Tween<Offset>(begin: const Offset(0, 0.2), end: Offset.zero)
        .animate(
          CurvedAnimation(
            parent: _controller,
            curve: const Interval(0.3, 0.8, curve: Curves.easeOutCubic),
          ),
        );
    _controller.forward();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AuroraBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        body: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: ResponsiveUtils.responsivePadding(context),
              child: FadeTransition(
                opacity: _fadeIn,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 520),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      PremiumLogo(size: 130),
                      const SizedBox(height: 28),
                      Text(
                        'منصة إدارة مشاريع التخرج',
                        style: Theme.of(context).textTheme.headlineLarge,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 10),
                      Text(
                        'جامعة الزهراء - كلية هندسة تكنولوجيا المعلومات',
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: AppColors.electricCyan,
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 48),
                      SlideTransition(
                        position: _slideUp,
                        child: Column(
                          children: [
                            _buildRoleButton(
                              context: context,
                              icon: Icons.admin_panel_settings,
                              title: 'دخول كمسؤول',
                              subtitle: 'ADMIN ACCESS',
                              gradient: AppColors.auroraGradient,
                              onTap: () => _navigateToAuth(UserRole.admin),
                            ),
                            const SizedBox(height: 16),
                            _buildRoleButton(
                              context: context,
                              icon: Icons.person_outline,
                              title: 'دخول كتدريسي',
                              subtitle: 'INSTRUCTOR ACCESS',
                              gradient: AppColors.violetPinkGradient,
                              onTap: () => _navigateToAuth(UserRole.instructor),
                            ),
                            const SizedBox(height: 16),
                            _buildRoleButton(
                              context: context,
                              icon: Icons.school_outlined,
                              title: 'دخول كطالبة',
                              subtitle: 'STUDENT ACCESS',
                              gradient: AppColors.pinkCyanGradient,
                              onTap: () => _navigateToAuth(UserRole.student),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 36),
                      TextButton(
                        onPressed: () {
                          Navigator.push(
                            context,
                            MaterialPageRoute(
                              builder: (context) => const AboutScreen(),
                            ),
                          );
                        },
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const Icon(
                              Icons.info_outline,
                              color: AppColors.primaryPink,
                              size: 16,
                            ),
                            const SizedBox(width: 8),
                            Text(
                              'عن المنصة والمصممة',
                              style: Theme.of(context).textTheme.bodyMedium
                                  ?.copyWith(
                                    color: AppColors.primaryPink,
                                    fontWeight: FontWeight.w600,
                                  ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildRoleButton({
    required BuildContext context,
    required IconData icon,
    required String title,
    required String subtitle,
    required Gradient gradient,
    required VoidCallback onTap,
  }) {
    return Container(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(20),
        gradient: gradient,
        boxShadow: [
          BoxShadow(
            color: AppColors.primaryPink.withOpacity(0.35),
            blurRadius: 25,
            offset: const Offset(0, 10),
          ),
          BoxShadow(
            color: AppColors.neonPurple.withOpacity(0.2),
            blurRadius: 30,
            offset: const Offset(0, 5),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(20),
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.all(22),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    color: Colors.white.withOpacity(0.2),
                    borderRadius: BorderRadius.circular(14),
                    border: Border.all(
                      color: Colors.white.withOpacity(0.3),
                      width: 1,
                    ),
                  ),
                  child: Icon(icon, color: Colors.white, size: 28),
                ),
                const SizedBox(width: 18),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        title,
                        style: Theme.of(context).textTheme.titleMedium
                            ?.copyWith(
                              color: Colors.white,
                              fontWeight: FontWeight.w700,
                            ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        subtitle,
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Colors.white.withOpacity(0.8),
                          letterSpacing: 2,
                          fontSize: 10,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                ),
                const Icon(Icons.arrow_back_ios, color: Colors.white, size: 16),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _navigateToAuth(UserRole role) {
    Navigator.push(
      context,
      PageRouteBuilder(
        pageBuilder: (context, animation, secondaryAnimation) =>
            AuthGate(role: role),
        transitionsBuilder: (context, animation, secondaryAnimation, child) {
          return FadeTransition(
            opacity: animation,
            child: SlideTransition(
              position: Tween<Offset>(
                begin: const Offset(0.05, 0),
                end: Offset.zero,
              ).animate(animation),
              child: child,
            ),
          );
        },
        transitionDuration: const Duration(milliseconds: 500),
      ),
    );
  }
}

// ============================================================================
// 💎 ABOUT SCREEN - بدون مميزات
// ============================================================================
class AboutScreen extends StatelessWidget {
  const AboutScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return AuroraBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: AppBar(
          title: const Text('عن المنصة'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.pop(context),
          ),
        ),
        body: SingleChildScrollView(
          padding: ResponsiveUtils.responsivePadding(context),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 800),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                NeonGlassCard(
                  margin: EdgeInsets.zero,
                  padding: const EdgeInsets.all(28),
                  glowColor: AppColors.primaryPink,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const SectionHeader(
                        title: 'عن المنصة',
                        subtitle: 'Platform Overview',
                        icon: Icons.info_outline,
                        iconGradient: AppColors.auroraGradient,
                      ),
                      const SizedBox(height: 20),
                      Text(
                        'منصة رقمية متكاملة لإدارة مشاريع التخرج الأكاديمية، صُممت خصيصاً لتلبية احتياجات البيئة الجامعية في جامعة الزهراء - كلية هندسة تكنولوجيا المعلومات - قسم الأمن السيبراني.',
                        style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: AppColors.secondaryText,
                          height: 1.8,
                        ),
                      ),
                      const SizedBox(height: 16),
                      Text(
                        'توفر المنصة بيئة آمنة ومنظمة لإدارة المشاريع، مع نظام صلاحيات متعدد المستويات، ونظام تحقق ذكي، وسجلات أمان مفصلة.',
                        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                          color: AppColors.mutedText,
                          height: 1.7,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 20),
                NeonGlassCard(
                  margin: EdgeInsets.zero,
                  padding: const EdgeInsets.all(28),
                  glowColor: AppColors.neonPurple,
                  child: Column(
                    children: [
                      const SectionHeader(
                        title: 'المصممة',
                        subtitle: 'Lead Designer',
                        icon: Icons.engineering,
                        iconGradient: AppColors.violetPinkGradient,
                      ),
                      const SizedBox(height: 28),
                      Container(
                        padding: const EdgeInsets.all(22),
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          gradient: AppColors.auroraGradient,
                          boxShadow: [
                            BoxShadow(
                              color: AppColors.primaryPink.withOpacity(0.6),
                              blurRadius: 40,
                              spreadRadius: 5,
                            ),
                            BoxShadow(
                              color: AppColors.neonPurple.withOpacity(0.4),
                              blurRadius: 50,
                              spreadRadius: 8,
                            ),
                          ],
                        ),
                        child: const Icon(
                          Icons.person,
                          color: Colors.white,
                          size: 55,
                        ),
                      ),
                      const SizedBox(height: 20),
                      Text(
                        'منصة إدارة مشاريع التخرج 2026',
                        style: Theme.of(context).textTheme.headlineMedium
                            ?.copyWith(
                              color: Colors.white,
                              fontWeight: FontWeight.w800,
                            ),
                      ),
                      const SizedBox(height: 6),
                      Text(
                        'Graduation Platform 2026',
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: AppColors.mutedText,
                          letterSpacing: 2,
                        ),
                      ),
                      const SizedBox(height: 24),
                      Container(
                        height: 1,
                        decoration: BoxDecoration(
                          gradient: LinearGradient(
                            colors: [
                              Colors.transparent,
                              Colors.white.withOpacity(0.2),
                              Colors.transparent,
                            ],
                          ),
                        ),
                      ),
                      const SizedBox(height: 20),
                      _buildInfoRow(context, Icons.school, 'جامعة الزهراء'),
                      _buildInfoRow(
                        context,
                        Icons.engineering,
                        'كلية هندسة تكنولوجيا المعلومات',
                      ),
                      _buildInfoRow(
                        context,
                        Icons.security,
                        'قسم الأمن السيبراني',
                      ),
                      _buildInfoRow(context, Icons.calendar_today, '2026'),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, IconData icon, String text) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              gradient: AppColors.auroraGradient,
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(icon, color: Colors.white, size: 16),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              text,
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(color: AppColors.secondaryText),
            ),
          ),
        ],
      ),
    );
  }
}

// ============================================================================
// 🔐 AUTH GATE
// ============================================================================
class AuthGate extends StatefulWidget {
  final UserRole role;
  const AuthGate({super.key, required this.role});

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate>
    with SingleTickerProviderStateMixin {
  final _nameController = TextEditingController();
  final _codeController = TextEditingController();
  bool _isLoading = false;
  String? _errorMessage;
  bool _obscureCode = true;
  late AnimationController _controller;
  late Animation<double> _fadeIn;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1000),
    );
    _fadeIn = Tween<double>(
      begin: 0.0,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeOut));
    _controller.forward();
  }

  @override
  void dispose() {
    _nameController.dispose();
    _codeController.dispose();
    _controller.dispose();
    super.dispose();
  }

  String get _title {
    switch (widget.role) {
      case UserRole.admin:
        return 'دخول المسؤول';
      case UserRole.instructor:
        return 'دخول التدريسي';
      case UserRole.student:
        return 'دخول الطالبة';
    }
  }

  String get _subtitle {
    switch (widget.role) {
      case UserRole.admin:
        return 'ADMIN AUTHENTICATION';
      case UserRole.instructor:
        return 'INSTRUCTOR AUTHENTICATION';
      case UserRole.student:
        return 'STUDENT AUTHENTICATION';
    }
  }

  String get _codeHint {
    switch (widget.role) {
      case UserRole.admin:
        return 'أدخل رمز المسؤول';
      case UserRole.instructor:
        return 'أدخل رمز التدريسي';
      case UserRole.student:
        return 'أدخل الرمز الخاص (6 أحرف)';
    }
  }

  Gradient get _roleGradient {
    switch (widget.role) {
      case UserRole.admin:
        return AppColors.auroraGradient;
      case UserRole.instructor:
        return AppColors.violetPinkGradient;
      case UserRole.student:
        return AppColors.pinkCyanGradient;
    }
  }

  Future<void> _authenticate() async {
    setState(() {
      _isLoading = true;
      _errorMessage = null;
    });

    await Future.delayed(const Duration(milliseconds: 800));

    final name = _nameController.text.trim();
    final code = _codeController.text.trim().toUpperCase();
    final deviceId = 'device_${DateTime.now().millisecondsSinceEpoch}';
    final deviceName = 'Web Browser - ${DateTime.now().millisecondsSinceEpoch}';
    const ipAddress = 'راجع سجل NullGate المحلي';

    if (name.isEmpty || code.isEmpty) {
      setState(() {
        _isLoading = false;
        _errorMessage = 'يرجى إدخال جميع البيانات المطلوبة';
      });
      return;
    }

    if (SecurityManager.isAccountBanned(name)) {
      await SecurityManager.logSecurityEvent(
        action: '🚫 محاولة دخول محظور',
        details: 'محاولة دخول من حساب محظور: $name',
        ip: ipAddress,
        device: deviceName,
        userName: name,
        severity: SecuritySeverity.critical,
      );
      setState(() {
        _isLoading = false;
        _errorMessage = 'هذا الحساب محظور. تواصل مع الإدارة';
      });
      return;
    }

    if (SecurityManager.isAccountLocked(deviceId)) {
      final remainingTime = SecurityManager.getRemainingLockTime(deviceId);
      setState(() {
        _isLoading = false;
        _errorMessage = 'تم قفل الحساب مؤقتاً. حاول بعد $remainingTime دقائق';
      });
      return;
    }

    User? user;

    if (widget.role == UserRole.admin && code == 'ADMIN') {
      user = User(
        name: name,
        role: UserRole.admin,
        deviceId: deviceId,
        ipAddress: ipAddress,
        deviceName: deviceName,
      );
    } else if (widget.role == UserRole.instructor && code == 'PRO') {
      user = User(
        name: name,
        role: UserRole.instructor,
        deviceId: deviceId,
        ipAddress: ipAddress,
        deviceName: deviceName,
      );
    } else if (widget.role == UserRole.student) {
      if (StudentData.validateCredentials(name, code)) {
        final studentCode = StudentData.findStudentByName(name);
        user = User(
          name: name,
          role: UserRole.student,
          studentCode: studentCode,
          deviceId: deviceId,
          ipAddress: ipAddress,
          deviceName: deviceName,
        );
      }
    }

    if (user != null) {
      AppState.currentUser = user;
      await SecurityManager.logSecurityEvent(
        action: '✅ دخول ناجح',
        details: 'دخول ${user.roleDisplayName}: $name',
        ip: ipAddress,
        device: deviceName,
        userName: name,
        severity: SecuritySeverity.info,
      );
      if (mounted) {
        Navigator.pushReplacement(
          context,
          PageRouteBuilder(
            pageBuilder: (context, animation, secondaryAnimation) =>
                const RoleDashboard(),
            transitionsBuilder:
                (context, animation, secondaryAnimation, child) {
                  return FadeTransition(
                    opacity: animation,
                    child: SlideTransition(
                      position: Tween<Offset>(
                        begin: const Offset(0.05, 0),
                        end: Offset.zero,
                      ).animate(animation),
                      child: child,
                    ),
                  );
                },
            transitionDuration: const Duration(milliseconds: 500),
          ),
        );
      }
    } else {
      await SecurityManager.recordFailedAttempt(
        deviceId,
        'محاولة دخول فاشلة - اسم: $name',
        name,
      );
      setState(() {
        _isLoading = false;
        _errorMessage = 'بيانات الدخول غير صحيحة';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AuroraBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        body: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: ResponsiveUtils.responsivePadding(context),
              child: FadeTransition(
                opacity: _fadeIn,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 480),
                  child: NeonGlassCard(
                    margin: EdgeInsets.zero,
                    padding: EdgeInsets.all(
                      ResponsiveUtils.responsiveWidth(
                        context,
                        mobile: 24,
                        tablet: 32,
                        desktop: 40,
                      ),
                    ),
                    glowColor: AppColors.primaryPink,
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          padding: const EdgeInsets.all(14),
                          decoration: BoxDecoration(
                            gradient: _roleGradient,
                            borderRadius: BorderRadius.circular(16),
                            boxShadow: [
                              BoxShadow(
                                color: AppColors.primaryPink.withOpacity(0.5),
                                blurRadius: 20,
                              ),
                            ],
                          ),
                          child: Icon(
                            widget.role == UserRole.admin
                                ? Icons.admin_panel_settings
                                : widget.role == UserRole.instructor
                                ? Icons.person
                                : Icons.school,
                            color: Colors.white,
                            size: 32,
                          ),
                        ),
                        const SizedBox(height: 20),
                        Text(
                          _title,
                          style: Theme.of(context).textTheme.headlineMedium,
                        ),
                        const SizedBox(height: 6),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 4,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.primaryPink.withOpacity(0.15),
                            borderRadius: BorderRadius.circular(20),
                            border: Border.all(
                              color: AppColors.primaryPink.withOpacity(0.3),
                            ),
                          ),
                          child: Text(
                            _subtitle,
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(
                                  color: AppColors.primaryPink,
                                  letterSpacing: 2,
                                  fontSize: 10,
                                  fontWeight: FontWeight.w600,
                                ),
                          ),
                        ),
                        const SizedBox(height: 28),
                        PremiumTextField(
                          controller: _nameController,
                          label: 'الاسم الكامل',
                          hint: 'أدخل اسمك الكامل',
                          icon: Icons.person_outline,
                        ),
                        const SizedBox(height: 16),
                        PremiumTextField(
                          controller: _codeController,
                          label: 'رمز الدخول',
                          hint: _codeHint,
                          icon: Icons.lock_outline,
                          obscureText: _obscureCode,
                          suffixIcon: IconButton(
                            icon: Icon(
                              _obscureCode
                                  ? Icons.visibility_off
                                  : Icons.visibility,
                              color: AppColors.electricCyan,
                            ),
                            onPressed: () =>
                                setState(() => _obscureCode = !_obscureCode),
                          ),
                        ),
                        if (_errorMessage != null) ...[
                          const SizedBox(height: 16),
                          PremiumAlert(message: _errorMessage!),
                        ],
                        const SizedBox(height: 28),
                        PremiumButton(
                          text: 'دخول',
                          icon: Icons.login_rounded,
                          loading: _isLoading,
                          onPressed: _authenticate,
                          gradient: _roleGradient,
                        ),
                        const SizedBox(height: 16),
                        TextButton(
                          onPressed: () => Navigator.pop(context),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              const Icon(
                                Icons.arrow_forward,
                                color: AppColors.mutedText,
                                size: 16,
                              ),
                              const SizedBox(width: 6),
                              Text(
                                'رجوع',
                                style: Theme.of(context).textTheme.bodyMedium
                                    ?.copyWith(color: AppColors.mutedText),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

// ============================================================================
// 🎭 ROLE DASHBOARD
// ============================================================================
class RoleDashboard extends StatelessWidget {
  const RoleDashboard({super.key});

  @override
  Widget build(BuildContext context) {
    final user = AppState.currentUser!;
    switch (user.role) {
      case UserRole.admin:
        return const AdminDashboard();
      case UserRole.instructor:
        return const InstructorDashboard();
      case UserRole.student:
        return const StudentDashboard();
    }
  }
}

// ============================================================================
// 👨‍💼 ADMIN DASHBOARD
// ============================================================================
class AdminDashboard extends StatefulWidget {
  const AdminDashboard({super.key});

  @override
  State<AdminDashboard> createState() => _AdminDashboardState();
}

class _AdminDashboardState extends State<AdminDashboard> {
  int _currentIndex = 0;

  final List<Destination> _destinations = const [
    Destination(
      icon: Icons.folder_outlined,
      selectedIcon: Icons.folder,
      label: 'المشاريع',
    ),
    Destination(
      icon: Icons.notifications_outlined,
      selectedIcon: Icons.notifications,
      label: 'التبليغات',
    ),
    Destination(
      icon: Icons.people_outline,
      selectedIcon: Icons.people,
      label: 'المستخدمين',
    ),
    Destination(
      icon: Icons.security_outlined,
      selectedIcon: Icons.security,
      label: 'الأمان',
    ),
  ];

  List<Widget> get _pages => const [
    AdminProjectsTab(),
    AdminNoticesTab(),
    AdminUsersTab(),
    AdminSecurityTab(),
  ];

  @override
  Widget build(BuildContext context) {
    final user = AppState.currentUser!;

    return AuroraBackground(
      child: PremiumScaffold(
        title: _destinations[_currentIndex].label,
        subtitle: Row(
          children: [
            BreathingOrb(size: 8, color: AppColors.success),
            const SizedBox(width: 8),
            Text(
              '${user.name} • إداري النظام',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: AppColors.electricCyan,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
        destinations: _destinations,
        bodies: _pages,
        currentIndex: _currentIndex,
        onDestinationSelected: (index) => setState(() => _currentIndex = index),
        actions: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            decoration: BoxDecoration(
              gradient: AppColors.auroraGradient,
              borderRadius: BorderRadius.circular(20),
              boxShadow: [
                BoxShadow(
                  color: AppColors.primaryPink.withOpacity(0.4),
                  blurRadius: 10,
                ),
              ],
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.person, color: Colors.white, size: 14),
                const SizedBox(width: 6),
                Text(
                  user.name,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.w600,
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          IconButton(
            icon: Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: AppColors.danger.withOpacity(0.15),
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: AppColors.danger.withOpacity(0.3)),
              ),
              child: const Icon(
                Icons.logout_rounded,
                color: AppColors.danger,
                size: 18,
              ),
            ),
            onPressed: _logout,
          ),
        ],
        floatingActionButton: _currentIndex == 1
            ? Container(
                decoration: BoxDecoration(
                  gradient: AppColors.auroraGradient,
                  borderRadius: BorderRadius.circular(20),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.primaryPink.withOpacity(0.5),
                      blurRadius: 20,
                      offset: const Offset(0, 8),
                    ),
                  ],
                ),
                child: FloatingActionButton(
                  onPressed: () {
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (context) => PublishNoticePage(),
                      ),
                    );
                  },
                  backgroundColor: Colors.transparent,
                  elevation: 0,
                  child: const Icon(Icons.add_rounded, color: Colors.white),
                ),
              )
            : null,
      ),
    );
  }

  Future<void> _logout() async {
    final user = AppState.currentUser!;
    await SecurityManager.logSecurityEvent(
      action: '🚪 تسجيل خروج',
      details: 'خروج إداري: ${user.name}',
      ip: user.ipAddress,
      device: user.deviceName,
      userName: user.name,
      severity: SecuritySeverity.info,
    );
    AppState.currentUser = null;
    if (mounted) {
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (context) => const WelcomeScreen()),
      );
    }
  }
}

// ============================================================================
// 👨‍🏫 INSTRUCTOR DASHBOARD
// ============================================================================
class InstructorDashboard extends StatefulWidget {
  const InstructorDashboard({super.key});

  @override
  State<InstructorDashboard> createState() => _InstructorDashboardState();
}

class _InstructorDashboardState extends State<InstructorDashboard> {
  int _currentIndex = 0;

  final List<Destination> _destinations = const [
    Destination(
      icon: Icons.folder_outlined,
      selectedIcon: Icons.folder,
      label: 'مشاريعي',
    ),
    Destination(
      icon: Icons.notifications_outlined,
      selectedIcon: Icons.notifications,
      label: 'التبليغات',
    ),
  ];

  List<Widget> get _pages => const [
    InstructorProjectsTab(),
    InstructorNoticesTab(),
  ];

  @override
  Widget build(BuildContext context) {
    final user = AppState.currentUser!;

    return AuroraBackground(
      child: PremiumScaffold(
        title: _destinations[_currentIndex].label,
        subtitle: Row(
          children: [
            BreathingOrb(size: 8, color: AppColors.success),
            const SizedBox(width: 8),
            Text(
              '${user.name} • تدريسي',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: AppColors.electricCyan,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
        destinations: _destinations,
        bodies: _pages,
        currentIndex: _currentIndex,
        onDestinationSelected: (index) => setState(() => _currentIndex = index),
        actions: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            decoration: BoxDecoration(
              gradient: AppColors.violetPinkGradient,
              borderRadius: BorderRadius.circular(20),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.person, color: Colors.white, size: 14),
                const SizedBox(width: 6),
                Text(
                  user.name,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.w600,
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          IconButton(
            icon: Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: AppColors.danger.withOpacity(0.15),
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: AppColors.danger.withOpacity(0.3)),
              ),
              child: const Icon(
                Icons.logout_rounded,
                color: AppColors.danger,
                size: 18,
              ),
            ),
            onPressed: _logout,
          ),
        ],
        floatingActionButton: Container(
          decoration: BoxDecoration(
            gradient: AppColors.violetPinkGradient,
            borderRadius: BorderRadius.circular(20),
            boxShadow: [
              BoxShadow(
                color: AppColors.neonPurple.withOpacity(0.5),
                blurRadius: 20,
                offset: const Offset(0, 8),
              ),
            ],
          ),
          child: FloatingActionButton(
            onPressed: () {
              if (_currentIndex == 0) {
                Navigator.push(
                  context,
                  MaterialPageRoute(builder: (context) => PublishProjectPage()),
                );
              } else {
                Navigator.push(
                  context,
                  MaterialPageRoute(builder: (context) => PublishNoticePage()),
                );
              }
            },
            backgroundColor: Colors.transparent,
            elevation: 0,
            child: const Icon(Icons.add_rounded, color: Colors.white),
          ),
        ),
      ),
    );
  }

  Future<void> _logout() async {
    final user = AppState.currentUser!;
    await SecurityManager.logSecurityEvent(
      action: '🚪 تسجيل خروج',
      details: 'خروج تدريسي: ${user.name}',
      ip: user.ipAddress,
      device: user.deviceName,
      userName: user.name,
      severity: SecuritySeverity.info,
    );
    AppState.currentUser = null;
    if (mounted) {
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (context) => const WelcomeScreen()),
      );
    }
  }
}

// ============================================================================
// 👩‍🎓 STUDENT DASHBOARD
// ============================================================================
class StudentDashboard extends StatefulWidget {
  const StudentDashboard({super.key});

  @override
  State<StudentDashboard> createState() => _StudentDashboardState();
}

class _StudentDashboardState extends State<StudentDashboard> {
  int _currentIndex = 0;

  final List<Destination> _destinations = const [
    Destination(
      icon: Icons.folder_outlined,
      selectedIcon: Icons.folder,
      label: 'المشاريع',
    ),
    Destination(
      icon: Icons.notifications_outlined,
      selectedIcon: Icons.notifications,
      label: 'التبليغات',
    ),
  ];

  List<Widget> get _pages => const [StudentProjectsTab(), StudentNoticesTab()];

  @override
  Widget build(BuildContext context) {
    final user = AppState.currentUser!;

    return AuroraBackground(
      child: PremiumScaffold(
        title: _destinations[_currentIndex].label,
        subtitle: Row(
          children: [
            BreathingOrb(size: 8, color: AppColors.success),
            const SizedBox(width: 8),
            Text(
              '${user.name} • ${user.studentCode}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: AppColors.electricCyan,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
        destinations: _destinations,
        bodies: _pages,
        currentIndex: _currentIndex,
        onDestinationSelected: (index) => setState(() => _currentIndex = index),
        actions: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
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
          const SizedBox(width: 12),
          IconButton(
            icon: Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: AppColors.danger.withOpacity(0.15),
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: AppColors.danger.withOpacity(0.3)),
              ),
              child: const Icon(
                Icons.logout_rounded,
                color: AppColors.danger,
                size: 18,
              ),
            ),
            onPressed: _logout,
          ),
        ],
      ),
    );
  }

  Future<void> _logout() async {
    final user = AppState.currentUser!;
    await SecurityManager.logSecurityEvent(
      action: '🚪 تسجيل خروج',
      details: 'خروج طالبة: ${user.name}',
      ip: user.ipAddress,
      device: user.deviceName,
      userName: user.name,
      severity: SecuritySeverity.info,
    );
    AppState.currentUser = null;
    if (mounted) {
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (context) => const WelcomeScreen()),
      );
    }
  }
}

// ============================================================================
// 📁 ADMIN PROJECTS TAB
// ============================================================================
class AdminProjectsTab extends StatefulWidget {
  const AdminProjectsTab({super.key});

  @override
  State<AdminProjectsTab> createState() => _AdminProjectsTabState();
}

class _AdminProjectsTabState extends State<AdminProjectsTab> {
  @override
  Widget build(BuildContext context) {
    final totalApplications = AppState.projects.fold(
      0,
      (sum, p) => sum + p.applications.length,
    );
    final activeProjects = AppState.projects
        .where((p) => p.status == 'active')
        .length;

    return SingleChildScrollView(
      padding: ResponsiveUtils.responsivePadding(context),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          PremiumGrid(
            children: [
              PremiumStatCard(
                label: 'إجمالي المشاريع',
                value: '${AppState.projects.length}',
                icon: Icons.folder,
                gradient: AppColors.auroraGradient,
                subtitle: 'جميع المشاريع المنشورة',
              ),
              PremiumStatCard(
                label: 'إجمالي الطلبات',
                value: '$totalApplications',
                icon: Icons.assignment,
                gradient: AppColors.pinkCyanGradient,
                subtitle: 'طلبات الانضمام',
              ),
              PremiumStatCard(
                label: 'المشاريع النشطة',
                value: '$activeProjects',
                icon: Icons.check_circle,
                gradient: AppColors.successGradient,
                subtitle: 'مشاريع قيد التنفيذ',
              ),
            ],
          ),
          const SizedBox(height: 32),
          const SectionHeader(
            title: 'جميع المشاريع',
            subtitle: 'All Projects',
            icon: Icons.folder_open,
            iconGradient: AppColors.auroraGradient,
          ),
          const SizedBox(height: 20),
          if (AppState.projects.isEmpty)
            _buildEmptyState(context, 'لا توجد مشاريع حالياً')
          else
            ListView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: AppState.projects.length,
              itemBuilder: (context, index) {
                final project = AppState.projects[index];
                return Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: NeonGlassCard(
                    margin: EdgeInsets.zero,
                    padding: const EdgeInsets.all(20),
                    glowColor: AppColors.primaryPink,
                    child: Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.all(14),
                          decoration: BoxDecoration(
                            gradient: AppColors.auroraGradient,
                            borderRadius: BorderRadius.circular(14),
                            boxShadow: [
                              BoxShadow(
                                color: AppColors.primaryPink.withOpacity(0.4),
                                blurRadius: 15,
                              ),
                            ],
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
                                project.title,
                                style: Theme.of(context).textTheme.titleMedium,
                              ),
                              const SizedBox(height: 6),
                              Row(
                                children: [
                                  PremiumBadge(
                                    text: project.supervisor,
                                    color: AppColors.electricCyan,
                                    icon: Icons.person,
                                  ),
                                  const SizedBox(width: 8),
                                  PremiumBadge(
                                    text: '${project.applications.length} طلب',
                                    color: AppColors.primaryPink,
                                    icon: Icons.assignment,
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                        IconButton(
                          icon: Container(
                            padding: const EdgeInsets.all(8),
                            decoration: BoxDecoration(
                              color: AppColors.danger.withOpacity(0.15),
                              borderRadius: BorderRadius.circular(10),
                              border: Border.all(
                                color: AppColors.danger.withOpacity(0.3),
                              ),
                            ),
                            child: const Icon(
                              Icons.delete_outline,
                              color: AppColors.danger,
                              size: 18,
                            ),
                          ),
                          onPressed: () async {
                            setState(() => AppState.projects.removeAt(index));
                            await AppState.saveProjects();
                          },
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
        ],
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context, String message) {
    return Container(
      padding: const EdgeInsets.all(40),
      child: Column(
        children: [
          Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              gradient: AppColors.auroraGradient,
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.folder_off, color: Colors.white, size: 40),
          ),
          const SizedBox(height: 16),
          Text(message, style: Theme.of(context).textTheme.titleMedium),
        ],
      ),
    );
  }
}

// ============================================================================
// 📢 ADMIN NOTICES TAB
// ============================================================================
class AdminNoticesTab extends StatefulWidget {
  const AdminNoticesTab({super.key});

  @override
  State<AdminNoticesTab> createState() => _AdminNoticesTabState();
}

class _AdminNoticesTabState extends State<AdminNoticesTab> {
  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: ResponsiveUtils.responsivePadding(context),
      itemCount: AppState.notices.length,
      itemBuilder: (context, index) {
        final notice = AppState.notices[index];
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: NeonGlassCard(
            margin: EdgeInsets.zero,
            padding: const EdgeInsets.all(22),
            glowColor: notice.pinned
                ? AppColors.primaryPink
                : AppColors.electricCyan,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        gradient: notice.pinned
                            ? AppColors.auroraGradient
                            : AppColors.pinkCyanGradient,
                        borderRadius: BorderRadius.circular(14),
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.primaryPink.withOpacity(0.4),
                            blurRadius: 15,
                          ),
                        ],
                      ),
                      child: Icon(
                        notice.pinned ? Icons.push_pin : Icons.campaign,
                        color: Colors.white,
                        size: 22,
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            notice.title,
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          const SizedBox(height: 4),
                          Text(
                            '${notice.createdAt.day}/${notice.createdAt.month}/${notice.createdAt.year}',
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(color: AppColors.mutedText),
                          ),
                        ],
                      ),
                    ),
                    IconButton(
                      icon: Container(
                        padding: const EdgeInsets.all(8),
                        decoration: BoxDecoration(
                          color: AppColors.danger.withOpacity(0.15),
                          borderRadius: BorderRadius.circular(10),
                          border: Border.all(
                            color: AppColors.danger.withOpacity(0.3),
                          ),
                        ),
                        child: const Icon(
                          Icons.delete_outline,
                          color: AppColors.danger,
                          size: 18,
                        ),
                      ),
                      onPressed: () async {
                        setState(() => AppState.notices.removeAt(index));
                        await AppState.saveNotices();
                      },
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                Text(
                  notice.content,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    PremiumBadge(
                      text: notice.authorRole,
                      color: AppColors.primaryPink,
                      icon: Icons.badge,
                    ),
                    const SizedBox(width: 8),
                    PremiumBadge(
                      text: notice.author,
                      color: AppColors.electricCyan,
                      icon: Icons.person,
                    ),
                  ],
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

// ============================================================================
// 👥 ADMIN USERS TAB
// ============================================================================
class AdminUsersTab extends StatefulWidget {
  const AdminUsersTab({super.key});

  @override
  State<AdminUsersTab> createState() => _AdminUsersTabState();
}

class _AdminUsersTabState extends State<AdminUsersTab> {
  @override
  Widget build(BuildContext context) {
    final banned = SecurityManager.bannedAccounts;
    final logs = SecurityManager.securityLogs;
    final uniqueUsers = logs.map((l) => l.userName).toSet().toList();

    return SingleChildScrollView(
      padding: ResponsiveUtils.responsivePadding(context),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          PremiumGrid(
            children: [
              PremiumStatCard(
                label: 'المستخدمين النشطين',
                value:
                    '${uniqueUsers.where((u) => !banned.contains(u.toLowerCase().trim())).length}',
                icon: Icons.person,
                gradient: AppColors.successGradient,
              ),
              PremiumStatCard(
                label: 'المحظورين',
                value: '${banned.length}',
                icon: Icons.block,
                gradient: AppColors.dangerGradient,
              ),
              PremiumStatCard(
                label: 'إجمالي السجلات',
                value: '${logs.length}',
                icon: Icons.list_alt,
                gradient: AppColors.auroraGradient,
              ),
            ],
          ),
          const SizedBox(height: 32),
          const SectionHeader(
            title: 'المستخدمين المسجلين',
            subtitle: 'Registered Users',
            icon: Icons.people,
            iconGradient: AppColors.auroraGradient,
          ),
          const SizedBox(height: 20),
          if (uniqueUsers.isEmpty)
            Container(
              padding: const EdgeInsets.all(40),
              alignment: Alignment.center,
              child: Text(
                'لا يوجد مستخدمين مسجلين',
                style: Theme.of(context).textTheme.bodyMedium,
              ),
            )
          else
            ListView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: uniqueUsers.length,
              itemBuilder: (context, index) {
                final userName = uniqueUsers[index];
                final isBanned = banned.contains(userName.toLowerCase().trim());
                final userLogs = logs
                    .where((l) => l.userName == userName)
                    .toList();
                final lastLogin = userLogs.isNotEmpty
                    ? userLogs.last.timestamp
                    : null;

                return Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: NeonGlassCard(
                    margin: EdgeInsets.zero,
                    padding: const EdgeInsets.all(18),
                    glowColor: isBanned
                        ? AppColors.danger
                        : AppColors.primaryPink,
                    child: Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.all(12),
                          decoration: BoxDecoration(
                            gradient: isBanned
                                ? AppColors.dangerGradient
                                : AppColors.auroraGradient,
                            borderRadius: BorderRadius.circular(14),
                            boxShadow: [
                              BoxShadow(
                                color:
                                    (isBanned
                                            ? AppColors.danger
                                            : AppColors.primaryPink)
                                        .withOpacity(0.4),
                                blurRadius: 15,
                              ),
                            ],
                          ),
                          child: Icon(
                            isBanned ? Icons.block : Icons.person,
                            color: Colors.white,
                            size: 22,
                          ),
                        ),
                        const SizedBox(width: 14),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                userName,
                                style: Theme.of(context).textTheme.titleMedium,
                              ),
                              const SizedBox(height: 4),
                              Text(
                                lastLogin != null
                                    ? 'آخر دخول: ${lastLogin.day}/${lastLogin.month} ${lastLogin.hour}:${lastLogin.minute.toString().padLeft(2, '0')}'
                                    : 'لا توجد بيانات',
                                style: Theme.of(context).textTheme.bodySmall,
                              ),
                            ],
                          ),
                        ),
                        IconButton(
                          icon: Container(
                            padding: const EdgeInsets.all(8),
                            decoration: BoxDecoration(
                              color:
                                  (isBanned
                                          ? AppColors.success
                                          : AppColors.danger)
                                      .withOpacity(0.15),
                              borderRadius: BorderRadius.circular(10),
                              border: Border.all(
                                color:
                                    (isBanned
                                            ? AppColors.success
                                            : AppColors.danger)
                                        .withOpacity(0.3),
                              ),
                            ),
                            child: Icon(
                              isBanned ? Icons.lock_open : Icons.block,
                              color: isBanned
                                  ? AppColors.success
                                  : AppColors.danger,
                              size: 18,
                            ),
                          ),
                          onPressed: () async {
                            if (isBanned) {
                              await SecurityManager.unbanAccount(userName);
                            } else {
                              await SecurityManager.banAccount(userName);
                            }
                            setState(() {});
                          },
                        ),
                        IconButton(
                          icon: Container(
                            padding: const EdgeInsets.all(8),
                            decoration: BoxDecoration(
                              color: AppColors.electricCyan.withOpacity(0.15),
                              borderRadius: BorderRadius.circular(10),
                              border: Border.all(
                                color: AppColors.electricCyan.withOpacity(0.3),
                              ),
                            ),
                            child: const Icon(
                              Icons.info_outline,
                              color: AppColors.electricCyan,
                              size: 18,
                            ),
                          ),
                          onPressed: () {
                            showDialog(
                              context: context,
                              builder: (context) => AlertDialog(
                                backgroundColor: AppColors.midnightBlue,
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(24),
                                  side: BorderSide(
                                    color: Colors.white.withOpacity(0.1),
                                  ),
                                ),
                                title: Text('معلومات: $userName'),
                                content: SingleChildScrollView(
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    mainAxisSize: MainAxisSize.min,
                                    children: userLogs.reversed.take(10).map((
                                      log,
                                    ) {
                                      return Padding(
                                        padding: const EdgeInsets.symmetric(
                                          vertical: 6,
                                        ),
                                        child: Text(
                                          '${log.action}\n${log.details}\n📍 ${log.ip} | 💻 ${log.device}\n⏰ ${log.timestamp}',
                                          style: Theme.of(
                                            context,
                                          ).textTheme.bodySmall,
                                        ),
                                      );
                                    }).toList(),
                                  ),
                                ),
                                actions: [
                                  TextButton(
                                    onPressed: () => Navigator.pop(context),
                                    child: const Text('إغلاق'),
                                  ),
                                ],
                              ),
                            );
                          },
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
        ],
      ),
    );
  }
}

// ============================================================================
// 🔒 ADMIN SECURITY TAB
// ============================================================================
class AdminSecurityTab extends StatelessWidget {
  const AdminSecurityTab({super.key});

  @override
  Widget build(BuildContext context) {
    final logs = SecurityManager.securityLogs.reversed.toList();
    final stats = SecurityManager.getSecurityStats();

    return SingleChildScrollView(
      padding: ResponsiveUtils.responsivePadding(context),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          PremiumGrid(
            children: [
              PremiumStatCard(
                label: 'السجلات',
                value: '${stats['totalLogs']}',
                icon: Icons.list_alt,
                gradient: AppColors.auroraGradient,
              ),
              PremiumStatCard(
                label: 'حرج',
                value: '${stats['criticalEvents']}',
                icon: Icons.error,
                gradient: AppColors.dangerGradient,
              ),
              PremiumStatCard(
                label: 'محظور',
                value: '${stats['bannedAccounts']}',
                icon: Icons.block,
                gradient: AppColors.violetPinkGradient,
              ),
            ],
          ),
          const SizedBox(height: 32),
          const SectionHeader(
            title: 'سجل الأحداث الأمنية',
            subtitle: 'Security Activity Log',
            icon: Icons.security,
            iconGradient: AppColors.auroraGradient,
          ),
          const SizedBox(height: 20),
          if (logs.isEmpty)
            Container(
              padding: const EdgeInsets.all(40),
              alignment: Alignment.center,
              child: Text(
                'لا توجد سجلات أمنية',
                style: Theme.of(context).textTheme.bodyMedium,
              ),
            )
          else
            ListView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: logs.length,
              itemBuilder: (context, index) {
                final log = logs[index];
                return Padding(
                  padding: const EdgeInsets.only(bottom: 10),
                  child: UltraGlassCard(
                    margin: EdgeInsets.zero,
                    padding: const EdgeInsets.all(16),
                    child: Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.all(10),
                          decoration: BoxDecoration(
                            color: _getSeverityColor(
                              log.severity,
                            ).withOpacity(0.15),
                            borderRadius: BorderRadius.circular(12),
                            border: Border.all(
                              color: _getSeverityColor(
                                log.severity,
                              ).withOpacity(0.3),
                            ),
                          ),
                          child: Icon(
                            _getSeverityIcon(log.severity),
                            color: _getSeverityColor(log.severity),
                            size: 20,
                          ),
                        ),
                        const SizedBox(width: 14),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                log.action,
                                style: Theme.of(context).textTheme.titleMedium
                                    ?.copyWith(fontWeight: FontWeight.bold),
                              ),
                              const SizedBox(height: 4),
                              Text(
                                log.details,
                                style: Theme.of(context).textTheme.bodySmall,
                              ),
                              const SizedBox(height: 6),
                              Row(
                                children: [
                                  Icon(
                                    Icons.person,
                                    color: AppColors.mutedText,
                                    size: 12,
                                  ),
                                  const SizedBox(width: 4),
                                  Text(
                                    log.userName,
                                    style: Theme.of(context).textTheme.bodySmall
                                        ?.copyWith(
                                          color: AppColors.mutedText,
                                          fontSize: 10,
                                        ),
                                  ),
                                  const SizedBox(width: 12),
                                  Icon(
                                    Icons.location_on,
                                    color: AppColors.mutedText,
                                    size: 12,
                                  ),
                                  const SizedBox(width: 4),
                                  Text(
                                    log.ip,
                                    style: Theme.of(context).textTheme.bodySmall
                                        ?.copyWith(
                                          color: AppColors.mutedText,
                                          fontSize: 10,
                                        ),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 10,
                            vertical: 4,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.electricCyan.withOpacity(0.15),
                            borderRadius: BorderRadius.circular(20),
                            border: Border.all(
                              color: AppColors.electricCyan.withOpacity(0.3),
                            ),
                          ),
                          child: Text(
                            '${log.timestamp.hour}:${log.timestamp.minute.toString().padLeft(2, '0')}',
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(
                                  color: AppColors.electricCyan,
                                  fontWeight: FontWeight.w600,
                                  fontSize: 11,
                                ),
                          ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
        ],
      ),
    );
  }

  IconData _getSeverityIcon(SecuritySeverity severity) {
    switch (severity) {
      case SecuritySeverity.info:
        return Icons.info_outline;
      case SecuritySeverity.warning:
        return Icons.warning_amber;
      case SecuritySeverity.error:
        return Icons.error_outline;
      case SecuritySeverity.critical:
        return Icons.dangerous;
    }
  }

  Color _getSeverityColor(SecuritySeverity severity) {
    switch (severity) {
      case SecuritySeverity.info:
        return AppColors.info;
      case SecuritySeverity.warning:
        return AppColors.warning;
      case SecuritySeverity.error:
        return AppColors.danger;
      case SecuritySeverity.critical:
        return AppColors.primaryPink;
    }
  }
}

// ============================================================================
// 📚 INSTRUCTOR PROJECTS TAB
// ============================================================================
class InstructorProjectsTab extends StatelessWidget {
  const InstructorProjectsTab({super.key});

  @override
  Widget build(BuildContext context) {
    final user = AppState.currentUser!;
    final projects = AppState.projects
        .where((p) => p.supervisor == user.name)
        .toList();

    return SingleChildScrollView(
      padding: ResponsiveUtils.responsivePadding(context),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          PremiumGrid(
            children: [
              PremiumStatCard(
                label: 'مشاريعي',
                value: '${projects.length}',
                icon: Icons.folder,
                gradient: AppColors.violetPinkGradient,
              ),
              PremiumStatCard(
                label: 'إجمالي الطلبات',
                value:
                    '${projects.fold(0, (sum, p) => sum + p.applications.length)}',
                icon: Icons.assignment,
                gradient: AppColors.pinkCyanGradient,
              ),
              PremiumStatCard(
                label: 'المقبولين',
                value:
                    '${projects.fold(0, (sum, p) => sum + p.acceptedStudents.length)}',
                icon: Icons.check_circle,
                gradient: AppColors.successGradient,
              ),
            ],
          ),
          const SizedBox(height: 32),
          const SectionHeader(
            title: 'مشاريعي',
            subtitle: 'My Projects',
            icon: Icons.folder_open,
            iconGradient: AppColors.violetPinkGradient,
          ),
          const SizedBox(height: 20),
          if (projects.isEmpty)
            Container(
              padding: const EdgeInsets.all(40),
              child: Column(
                children: [
                  Container(
                    padding: const EdgeInsets.all(20),
                    decoration: BoxDecoration(
                      gradient: AppColors.violetPinkGradient,
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(
                      Icons.folder_off,
                      color: Colors.white,
                      size: 40,
                    ),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    'لا توجد مشاريع حالياً',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ],
              ),
            )
          else
            ListView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: projects.length,
              itemBuilder: (context, index) {
                final project = projects[index];
                return Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: NeonGlassCard(
                    margin: EdgeInsets.zero,
                    padding: const EdgeInsets.all(20),
                    glowColor: AppColors.neonPurple,
                    onTap: () {
                      Navigator.push(
                        context,
                        MaterialPageRoute(
                          builder: (context) =>
                              ProjectDetailPage(project: project),
                        ),
                      );
                    },
                    child: Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.all(14),
                          decoration: BoxDecoration(
                            gradient: AppColors.violetPinkGradient,
                            borderRadius: BorderRadius.circular(14),
                            boxShadow: [
                              BoxShadow(
                                color: AppColors.neonPurple.withOpacity(0.4),
                                blurRadius: 15,
                              ),
                            ],
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
                                project.title,
                                style: Theme.of(context).textTheme.titleMedium,
                              ),
                              const SizedBox(height: 6),
                              Row(
                                children: [
                                  PremiumBadge(
                                    text: '${project.applications.length} طلب',
                                    color: AppColors.primaryPink,
                                    icon: Icons.assignment,
                                  ),
                                  const SizedBox(width: 8),
                                  PremiumBadge(
                                    text:
                                        '${project.acceptedStudents.length} مقبول',
                                    color: AppColors.success,
                                    icon: Icons.check,
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                        PremiumBadge(
                          text: project.status,
                          color: AppColors.neonPurple,
                          outlined: true,
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
        ],
      ),
    );
  }
}

// ============================================================================
// 📢 INSTRUCTOR NOTICES TAB
// ============================================================================
class InstructorNoticesTab extends StatelessWidget {
  const InstructorNoticesTab({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: ResponsiveUtils.responsivePadding(context),
      itemCount: AppState.notices.length,
      itemBuilder: (context, index) {
        final notice = AppState.notices[index];
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: NeonGlassCard(
            margin: EdgeInsets.zero,
            padding: const EdgeInsets.all(22),
            glowColor: notice.pinned
                ? AppColors.neonPurple
                : AppColors.electricCyan,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        gradient: notice.pinned
                            ? AppColors.violetPinkGradient
                            : AppColors.pinkCyanGradient,
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: Icon(
                        notice.pinned ? Icons.push_pin : Icons.campaign,
                        color: Colors.white,
                        size: 22,
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Text(
                        notice.title,
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                Text(
                  notice.content,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    PremiumBadge(
                      text: notice.author,
                      color: AppColors.electricCyan,
                      icon: Icons.person,
                    ),
                    const SizedBox(width: 8),
                    PremiumBadge(
                      text: '${notice.createdAt.day}/${notice.createdAt.month}',
                      color: AppColors.mutedText,
                      icon: Icons.calendar_today,
                    ),
                  ],
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

// ============================================================================
// 🎓 STUDENT PROJECTS TAB
// ============================================================================
class StudentProjectsTab extends StatelessWidget {
  const StudentProjectsTab({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: ResponsiveUtils.responsivePadding(context),
      itemCount: AppState.projects.length,
      itemBuilder: (context, index) {
        final project = AppState.projects[index];
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: NeonGlassCard(
            margin: EdgeInsets.zero,
            padding: const EdgeInsets.all(20),
            glowColor: AppColors.electricCyan,
            onTap: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => ProjectDetailPage(project: project),
                ),
              );
            },
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    gradient: AppColors.pinkCyanGradient,
                    borderRadius: BorderRadius.circular(14),
                    boxShadow: [
                      BoxShadow(
                        color: AppColors.electricCyan.withOpacity(0.4),
                        blurRadius: 15,
                      ),
                    ],
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
                        project.title,
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: 6),
                      PremiumBadge(
                        text: 'المشرف: ${project.supervisor}',
                        color: AppColors.electricCyan,
                        icon: Icons.person,
                      ),
                    ],
                  ),
                ),
                Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: AppColors.primaryPink.withOpacity(0.15),
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(
                      color: AppColors.primaryPink.withOpacity(0.3),
                    ),
                  ),
                  child: const Icon(
                    Icons.arrow_back_ios,
                    color: AppColors.primaryPink,
                    size: 14,
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

// ============================================================================
// 📢 STUDENT NOTICES TAB
// ============================================================================
class StudentNoticesTab extends StatelessWidget {
  const StudentNoticesTab({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: ResponsiveUtils.responsivePadding(context),
      itemCount: AppState.notices.length,
      itemBuilder: (context, index) {
        final notice = AppState.notices[index];
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: NeonGlassCard(
            margin: EdgeInsets.zero,
            padding: const EdgeInsets.all(22),
            glowColor: notice.pinned
                ? AppColors.primaryPink
                : AppColors.electricCyan,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        gradient: notice.pinned
                            ? AppColors.auroraGradient
                            : AppColors.pinkCyanGradient,
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: Icon(
                        notice.pinned ? Icons.push_pin : Icons.campaign,
                        color: Colors.white,
                        size: 22,
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Text(
                        notice.title,
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                Text(
                  notice.content,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    PremiumBadge(
                      text: notice.author,
                      color: AppColors.electricCyan,
                      icon: Icons.person,
                    ),
                    const SizedBox(width: 8),
                    PremiumBadge(
                      text: '${notice.createdAt.day}/${notice.createdAt.month}',
                      color: AppColors.mutedText,
                      icon: Icons.calendar_today,
                    ),
                  ],
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
