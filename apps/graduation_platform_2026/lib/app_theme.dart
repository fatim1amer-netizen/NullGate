import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

class AppColors {
  // ===== Premium Color Palette - Dark Violet + Pink Gradient + Cyan Glow =====
  static const Color primaryPink = Color(0xFFFF2D95);
  static const Color hotPink = Color(0xFFFF5FA2);
  static const Color softPink = Color(0xFFFFC1DC);
  static const Color roseGold = Color(0xFFE8B4B8);

  static const Color electricCyan = Color(0xFF00E5FF);
  static const Color skyBlue = Color(0xFF4FC3F7);
  static const Color deepCyan = Color(0xFF0891B2);

  static const Color neonPurple = Color(0xFFB026FF);
  static const Color darkViolet = Color(0xFF6D28D9);
  static const Color softViolet = Color(0xFFA78BFA);

  static const Color obsidian = Color(0xFF050816);
  static const Color deepNavy = Color(0xFF0A0E27);
  static const Color midnightBlue = Color(0xFF0F172A);
  static const Color charcoal = Color(0xFF1A1F3A);
  static const Color graphite = Color(0xFF252B48);

  // Status Colors
  static const Color success = Color(0xFF10B981);
  static const Color emerald = Color(0xFF059669);
  static const Color warning = Color(0xFFF59E0B);
  static const Color danger = Color(0xFFEF4444);
  static const Color info = Color(0xFF3B82F6);

  // Text Colors
  static const Color whiteText = Color(0xFFFFFFFF);
  static const Color secondaryText = Color(0xFFCBD5E1);
  static const Color mutedText = Color(0xFF94A3B8);
  static const Color dimText = Color(0xFF64748B);

  // ===== Premium Gradients =====
  static const LinearGradient auroraGradient = LinearGradient(
    colors: [Color(0xFFFF2D95), Color(0xFFB026FF), Color(0xFF4FC3F7)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    stops: [0.0, 0.5, 1.0],
  );

  static const Gradient cyberGradient = LinearGradient(
    colors: [Color(0xFF050816), Color(0xFF0A0E27), Color(0xFF0F172A)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient pinkCyanGradient = LinearGradient(
    colors: [Color(0xFFFF2D95), Color(0xFF00E5FF)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient violetPinkGradient = LinearGradient(
    colors: [Color(0xFFB026FF), Color(0xFFFF2D95)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const Gradient neonGlowGradient = LinearGradient(
    colors: [Color(0xFFFF2D95), Color(0xFFB026FF), Color(0xFF00E5FF)],
    begin: Alignment.centerLeft,
    end: Alignment.centerRight,
  );

  static const LinearGradient liquidGlass = LinearGradient(
    colors: [Color(0x1AFFFFFF), Color(0x0DFFFFFF), Color(0x05FFFFFF)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient auroraBackground = LinearGradient(
    colors: [
      Color(0xFF050816),
      Color(0xFF0A0E27),
      Color(0xFF1A0B2E),
      Color(0xFF0F172A),
    ],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    stops: [0.0, 0.4, 0.7, 1.0],
  );

  static const LinearGradient roseGoldGradient = LinearGradient(
    colors: [Color(0xFFE8B4B8), Color(0xFFFF2D95)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient electricGradient = LinearGradient(
    colors: [Color(0xFF00E5FF), Color(0xFF4FC3F7), Color(0xFFB026FF)],
    begin: Alignment.centerLeft,
    end: Alignment.centerRight,
  );

  static const LinearGradient successGradient = LinearGradient(
    colors: [Color(0xFF10B981), Color(0xFF059669)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient dangerGradient = LinearGradient(
    colors: [Color(0xFFEF4444), Color(0xFFDC2626)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient warningGradient = LinearGradient(
    colors: [Color(0xFFF59E0B), Color(0xFFD97706)],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );
}

class AppTheme {
  static ThemeData get cyberTheme {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: AppColors.obsidian,
      primaryColor: AppColors.primaryPink,

      colorScheme: const ColorScheme.dark(
        primary: AppColors.primaryPink,
        secondary: AppColors.neonPurple,
        tertiary: AppColors.electricCyan,
        surface: AppColors.midnightBlue,
        error: AppColors.danger,
        onPrimary: Colors.white,
        onSecondary: Colors.white,
        onSurface: AppColors.whiteText,
        outline: Color(0xFF1E293B),
      ),

      textTheme: TextTheme(
        displayLarge: GoogleFonts.cairo(
          fontSize: 36,
          fontWeight: FontWeight.w800,
          color: AppColors.whiteText,
          letterSpacing: -0.5,
          height: 1.2,
        ),
        displayMedium: GoogleFonts.cairo(
          fontSize: 30,
          fontWeight: FontWeight.w700,
          color: AppColors.whiteText,
          letterSpacing: -0.3,
        ),
        headlineLarge: GoogleFonts.cairo(
          fontSize: 26,
          fontWeight: FontWeight.w700,
          color: AppColors.whiteText,
        ),
        headlineMedium: GoogleFonts.cairo(
          fontSize: 22,
          fontWeight: FontWeight.w600,
          color: AppColors.whiteText,
        ),
        titleLarge: GoogleFonts.cairo(
          fontSize: 18,
          fontWeight: FontWeight.w600,
          color: AppColors.whiteText,
        ),
        titleMedium: GoogleFonts.cairo(
          fontSize: 16,
          fontWeight: FontWeight.w500,
          color: AppColors.whiteText,
        ),
        bodyLarge: GoogleFonts.cairo(
          fontSize: 16,
          color: AppColors.secondaryText,
          height: 1.6,
        ),
        bodyMedium: GoogleFonts.cairo(
          fontSize: 14,
          color: AppColors.secondaryText,
          height: 1.5,
        ),
        bodySmall: GoogleFonts.cairo(fontSize: 12, color: AppColors.mutedText),
        labelLarge: GoogleFonts.poppins(
          fontSize: 14,
          fontWeight: FontWeight.w600,
          color: Colors.white,
          letterSpacing: 0.8,
        ),
      ),

      appBarTheme: AppBarTheme(
        backgroundColor: Colors.transparent,
        elevation: 0,
        centerTitle: true,
        iconTheme: const IconThemeData(color: AppColors.primaryPink, size: 24),
        titleTextStyle: GoogleFonts.cairo(
          color: AppColors.whiteText,
          fontSize: 20,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.5,
        ),
      ),

      cardTheme: CardThemeData(
        color: AppColors.midnightBlue,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(24),
          side: BorderSide(color: Colors.white.withOpacity(0.08), width: 1),
        ),
      ),

      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: AppColors.primaryPink,
          foregroundColor: Colors.white,
          padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
          ),
          elevation: 0,
          textStyle: GoogleFonts.cairo(
            fontSize: 15,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),

      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: AppColors.midnightBlue,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(16),
          borderSide: BorderSide(
            color: Colors.white.withOpacity(0.1),
            width: 1.5,
          ),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(16),
          borderSide: BorderSide(
            color: Colors.white.withOpacity(0.1),
            width: 1.5,
          ),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(16),
          borderSide: const BorderSide(color: AppColors.primaryPink, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(16),
          borderSide: const BorderSide(color: AppColors.danger, width: 1.5),
        ),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 20,
          vertical: 18,
        ),
        labelStyle: GoogleFonts.cairo(color: AppColors.mutedText, fontSize: 14),
        hintStyle: GoogleFonts.cairo(
          color: AppColors.dimText.withOpacity(0.6),
          fontSize: 14,
        ),
        prefixIconColor: AppColors.primaryPink,
        suffixIconColor: AppColors.electricCyan,
      ),

      bottomNavigationBarTheme: BottomNavigationBarThemeData(
        backgroundColor: AppColors.midnightBlue,
        selectedItemColor: AppColors.primaryPink,
        unselectedItemColor: AppColors.mutedText,
        type: BottomNavigationBarType.fixed,
        elevation: 0,
        selectedLabelStyle: GoogleFonts.cairo(
          fontSize: 11,
          fontWeight: FontWeight.w600,
        ),
        unselectedLabelStyle: GoogleFonts.cairo(fontSize: 11),
      ),

      snackBarTheme: SnackBarThemeData(
        backgroundColor: AppColors.graphite,
        contentTextStyle: GoogleFonts.cairo(
          color: AppColors.whiteText,
          fontSize: 14,
        ),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        behavior: SnackBarBehavior.floating,
      ),

      dividerTheme: DividerThemeData(
        color: Colors.white.withOpacity(0.08),
        thickness: 1,
        space: 24,
      ),
    );
  }
}

// Spacing System (8pt)
class AppSpacing {
  static const double xs = 4;
  static const double sm = 8;
  static const double md = 16;
  static const double lg = 24;
  static const double xl = 32;
  static const double xxl = 48;
  static const double xxxl = 64;
}

// Border Radius System
class AppRadius {
  static const double sm = 8;
  static const double md = 12;
  static const double lg = 16;
  static const double xl = 20;
  static const double xxl = 24;
  static const double circle = 100;
}
