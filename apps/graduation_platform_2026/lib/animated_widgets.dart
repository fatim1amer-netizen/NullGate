import 'package:flutter/material.dart';
import 'dart:math' as math;
import 'app_theme.dart';

// ===== Aurora Background - Premium Multi-Layer =====
class AuroraBackground extends StatefulWidget {
  final Widget child;
  final bool showAurora;
  final bool showParticles;
  final bool showGrid;
  final bool showScanLine;

  const AuroraBackground({
    super.key,
    required this.child,
    this.showAurora = true,
    this.showParticles = true,
    this.showGrid = true,
    this.showScanLine = true,
  });

  @override
  State<AuroraBackground> createState() => _AuroraBackgroundState();
}

class _AuroraBackgroundState extends State<AuroraBackground>
    with TickerProviderStateMixin {
  late AnimationController _auroraController;
  late AnimationController _particlesController;
  late AnimationController _scanController;

  @override
  void initState() {
    super.initState();
    _auroraController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 20),
    )..repeat();

    _particlesController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 30),
    )..repeat();

    _scanController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 8),
    )..repeat();
  }

  @override
  void dispose() {
    _auroraController.dispose();
    _particlesController.dispose();
    _scanController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        // Base gradient
        Container(
          decoration: const BoxDecoration(gradient: AppColors.auroraBackground),
        ),

        // Aurora lights layer
        if (widget.showAurora)
          AnimatedBuilder(
            animation: _auroraController,
            builder: (context, child) {
              return CustomPaint(
                painter: AuroraPainter(_auroraController.value),
                child: child,
              );
            },
          ),

        // Grid layer
        if (widget.showGrid)
          AnimatedBuilder(
            animation: _scanController,
            builder: (context, child) {
              return CustomPaint(
                painter: CyberGridPainter(_scanController.value),
                child: child,
              );
            },
          ),

        // Particles layer
        if (widget.showParticles)
          AnimatedBuilder(
            animation: _particlesController,
            builder: (context, child) {
              return CustomPaint(
                painter: ParticlesPainter(_particlesController.value),
                child: child,
              );
            },
          ),

        // Scan line
        if (widget.showScanLine)
          AnimatedBuilder(
            animation: _scanController,
            builder: (context, child) {
              return CustomPaint(
                painter: ScanLinePainter(_scanController.value),
                child: child,
              );
            },
          ),

        // Content
        widget.child,
      ],
    );
  }
}

// ===== Aurora Painter - Liquid Lights =====
class AuroraPainter extends CustomPainter {
  final double animationValue;
  AuroraPainter(this.animationValue);

  @override
  void paint(Canvas canvas, Size size) {
    final blobs = [
      _AuroraBlob(
        baseCenter: Offset(size.width * 0.15, size.height * 0.25),
        color: AppColors.primaryPink.withOpacity(0.25),
        radius: size.width * 0.4,
        phase: 0,
        speed: 1.0,
      ),
      _AuroraBlob(
        baseCenter: Offset(size.width * 0.85, size.height * 0.35),
        color: AppColors.neonPurple.withOpacity(0.22),
        radius: size.width * 0.45,
        phase: 1.5,
        speed: 0.8,
      ),
      _AuroraBlob(
        baseCenter: Offset(size.width * 0.5, size.height * 0.75),
        color: AppColors.electricCyan.withOpacity(0.18),
        radius: size.width * 0.38,
        phase: 3.0,
        speed: 1.2,
      ),
      _AuroraBlob(
        baseCenter: Offset(size.width * 0.3, size.height * 0.6),
        color: AppColors.darkViolet.withOpacity(0.2),
        radius: size.width * 0.35,
        phase: 4.5,
        speed: 0.9,
      ),
    ];

    for (final blob in blobs) {
      final t = animationValue * 2 * math.pi * blob.speed;
      final offset = Offset(
        blob.baseCenter.dx + math.sin(t + blob.phase) * 50,
        blob.baseCenter.dy + math.cos(t * 0.7 + blob.phase) * 40,
      );

      final paint = Paint()
        ..shader = RadialGradient(
          colors: [
            blob.color,
            blob.color.withOpacity(0.5),
            blob.color.withOpacity(0),
          ],
          stops: const [0.0, 0.4, 1.0],
        ).createShader(Rect.fromCircle(center: offset, radius: blob.radius));

      canvas.drawCircle(offset, blob.radius, paint);
    }
  }

  @override
  bool shouldRepaint(AuroraPainter oldDelegate) => true;
}

class _AuroraBlob {
  final Offset baseCenter;
  final Color color;
  final double radius;
  final double phase;
  final double speed;

  _AuroraBlob({
    required this.baseCenter,
    required this.color,
    required this.radius,
    required this.phase,
    required this.speed,
  });
}

// ===== Cyber Grid Painter =====
class CyberGridPainter extends CustomPainter {
  final double animationValue;
  CyberGridPainter(this.animationValue);

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.white.withOpacity(0.04)
      ..strokeWidth = 0.5;

    const spacing = 60.0;
    final offsetY = (animationValue * spacing) % spacing;

    for (double x = 0; x < size.width; x += spacing) {
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), paint);
    }
    for (double y = -spacing + offsetY; y < size.height; y += spacing) {
      canvas.drawLine(Offset(0, y), Offset(size.width, y), paint);
    }
  }

  @override
  bool shouldRepaint(CyberGridPainter oldDelegate) => true;
}

// ===== Particles Painter - Floating Constellation =====
class ParticlesPainter extends CustomPainter {
  final double animationValue;
  ParticlesPainter(this.animationValue);

  @override
  void paint(Canvas canvas, Size size) {
    final particlePaint = Paint()..style = PaintingStyle.fill;
    final linePaint = Paint()
      ..strokeWidth = 0.5
      ..style = PaintingStyle.stroke;

    final particles = <Offset>[];

    // Draw particles
    for (int i = 0; i < 40; i++) {
      final t = animationValue * 2 * math.pi + i * 0.7;
      final x = (math.sin(t * 0.5 + i) * 0.5 + 0.5) * size.width;
      final y = (math.cos(t * 0.3 + i * 1.3) * 0.5 + 0.5) * size.height;
      final alpha = (math.sin(t + i) * 0.5 + 0.5);

      final color = i % 4 == 0
          ? AppColors.primaryPink.withOpacity(alpha * 0.8)
          : i % 4 == 1
          ? AppColors.electricCyan.withOpacity(alpha * 0.7)
          : i % 4 == 2
          ? AppColors.neonPurple.withOpacity(alpha * 0.6)
          : AppColors.softPink.withOpacity(alpha * 0.5);

      particlePaint.color = color;
      final offset = Offset(x, y);
      particles.add(offset);
      canvas.drawCircle(offset, 1.5 + alpha * 1.5, particlePaint);

      // Glow effect
      final glowPaint = Paint()
        ..color = color.withOpacity(alpha * 0.3)
        ..maskFilter = const MaskFilter.blur(BlurStyle.normal, 4);
      canvas.drawCircle(offset, 3 + alpha * 2, glowPaint);
    }

    // Draw constellation lines between close particles
    linePaint.color = Colors.white.withOpacity(0.06);
    for (int i = 0; i < particles.length; i++) {
      for (int j = i + 1; j < particles.length; j++) {
        final distance = (particles[i] - particles[j]).distance;
        if (distance < 120) {
          final opacity = (1 - distance / 120) * 0.15;
          linePaint.color = Colors.white.withOpacity(opacity);
          canvas.drawLine(particles[i], particles[j], linePaint);
        }
      }
    }
  }

  @override
  bool shouldRepaint(ParticlesPainter oldDelegate) => true;
}

// ===== Scan Line Painter =====
class ScanLinePainter extends CustomPainter {
  final double animationValue;
  ScanLinePainter(this.animationValue);

  @override
  void paint(Canvas canvas, Size size) {
    if (size.isEmpty) return;
    final y = (animationValue * size.height * 1.5) % size.height;

    // Main scan line
    final scanPaint = Paint()
      ..shader = LinearGradient(
        colors: [
          AppColors.primaryPink.withOpacity(0),
          AppColors.primaryPink.withOpacity(0.4),
          AppColors.electricCyan.withOpacity(0.4),
          AppColors.primaryPink.withOpacity(0),
        ],
        stops: const [0.0, 0.3, 0.7, 1.0],
      ).createShader(Rect.fromLTWH(0, y - 40, size.width, 80));

    canvas.drawRect(Rect.fromLTWH(0, y - 40, size.width, 80), scanPaint);
  }

  @override
  bool shouldRepaint(ScanLinePainter oldDelegate) => true;
}

// ===== Premium Animated Logo - Liquid Glass =====
class PremiumLogo extends StatefulWidget {
  final double size;
  const PremiumLogo({super.key, this.size = 120});

  @override
  State<PremiumLogo> createState() => _PremiumLogoState();
}

class _PremiumLogoState extends State<PremiumLogo>
    with TickerProviderStateMixin {
  late AnimationController _pulseController;
  late AnimationController _rotateController;
  late AnimationController _glowController;

  late Animation<double> _pulse;
  late Animation<double> _rotate;
  late Animation<double> _glow;

  @override
  void initState() {
    super.initState();

    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 3),
    );
    _pulse = Tween<double>(begin: 1.0, end: 1.08).animate(
      CurvedAnimation(parent: _pulseController, curve: Curves.easeInOut),
    );
    _pulseController.repeat(reverse: true);

    _rotateController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 20),
    )..repeat();
    _rotate = Tween<double>(
      begin: 0.0,
      end: 2 * math.pi,
    ).animate(CurvedAnimation(parent: _rotateController, curve: Curves.linear));

    _glowController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 2),
    );
    _glow = Tween<double>(begin: 0.5, end: 1.0).animate(
      CurvedAnimation(parent: _glowController, curve: Curves.easeInOut),
    );
    _glowController.repeat(reverse: true);
  }

  @override
  void dispose() {
    _pulseController.dispose();
    _rotateController.dispose();
    _glowController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: Listenable.merge([
        _pulseController,
        _rotateController,
        _glowController,
      ]),
      builder: (context, child) {
        return Transform.scale(
          scale: _pulse.value,
          child: Stack(
            alignment: Alignment.center,
            children: [
              // Outer glow ring
              Container(
                width: widget.size * 1.4,
                height: widget.size * 1.4,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  gradient: SweepGradient(
                    colors: [
                      AppColors.primaryPink.withOpacity(0.0),
                      AppColors.primaryPink.withOpacity(0.5 * _glow.value),
                      AppColors.neonPurple.withOpacity(0.5 * _glow.value),
                      AppColors.electricCyan.withOpacity(0.5 * _glow.value),
                      AppColors.primaryPink.withOpacity(0.0),
                    ],
                    stops: const [0.0, 0.25, 0.5, 0.75, 1.0],
                    transform: GradientRotation(_rotate.value),
                  ),
                ),
              ),

              // Main logo
              Container(
                width: widget.size,
                height: widget.size,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  gradient: AppColors.auroraGradient,
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.primaryPink.withOpacity(
                        0.6 * _glow.value,
                      ),
                      blurRadius: 40 * _glow.value,
                      spreadRadius: 8,
                    ),
                    BoxShadow(
                      color: AppColors.neonPurple.withOpacity(
                        0.4 * _glow.value,
                      ),
                      blurRadius: 60 * _glow.value,
                      spreadRadius: 12,
                    ),
                    BoxShadow(
                      color: AppColors.electricCyan.withOpacity(
                        0.3 * _glow.value,
                      ),
                      blurRadius: 80 * _glow.value,
                      spreadRadius: 16,
                    ),
                  ],
                ),
                child: Icon(
                  Icons.school_rounded,
                  color: Colors.white,
                  size: widget.size * 0.5,
                  shadows: [
                    Shadow(
                      color: Colors.white.withOpacity(0.8),
                      blurRadius: 10,
                    ),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

// ===== Breathing Orb - Small animated indicator =====
class BreathingOrb extends StatefulWidget {
  final double size;
  final Color color;
  const BreathingOrb({super.key, this.size = 12, required this.color});

  @override
  State<BreathingOrb> createState() => _BreathingOrbState();
}

class _BreathingOrbState extends State<BreathingOrb>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _pulse;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 2),
    );
    _pulse = Tween<double>(
      begin: 0.7,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeInOut));
    _controller.repeat(reverse: true);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, child) {
        return Container(
          width: widget.size,
          height: widget.size,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: widget.color,
            boxShadow: [
              BoxShadow(
                color: widget.color.withOpacity(0.6 * _pulse.value),
                blurRadius: 10 * _pulse.value,
                spreadRadius: 2 * _pulse.value,
              ),
            ],
          ),
        );
      },
    );
  }
}
