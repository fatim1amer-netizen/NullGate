import 'package:flutter/material.dart';
import 'app_theme.dart';

enum ScreenType { mobile, tablet, desktop }

class Destination {
  final IconData icon;
  final IconData selectedIcon;
  final String label;

  const Destination({
    required this.icon,
    required this.selectedIcon,
    required this.label,
  });
}

class ResponsiveUtils {
  static const double mobileBreakpoint = 600;
  static const double tabletBreakpoint = 1024;

  static ScreenType getScreenType(BuildContext context) {
    final width = MediaQuery.of(context).size.width;
    if (width < mobileBreakpoint) return ScreenType.mobile;
    if (width < tabletBreakpoint) return ScreenType.tablet;
    return ScreenType.desktop;
  }

  static double responsiveWidth(
    BuildContext context, {
    required double mobile,
    required double tablet,
    required double desktop,
  }) {
    final type = getScreenType(context);
    switch (type) {
      case ScreenType.mobile:
        return mobile;
      case ScreenType.tablet:
        return tablet;
      case ScreenType.desktop:
        return desktop;
    }
  }

  static EdgeInsets responsivePadding(BuildContext context) {
    final type = getScreenType(context);
    switch (type) {
      case ScreenType.mobile:
        return const EdgeInsets.all(16);
      case ScreenType.tablet:
        return const EdgeInsets.all(24);
      case ScreenType.desktop:
        return const EdgeInsets.all(32);
    }
  }
}

// ===== Premium Responsive Scaffold =====
class PremiumScaffold extends StatelessWidget {
  final String title;
  final Widget? subtitle;
  final List<Destination> destinations;
  final List<Widget> bodies;
  final int currentIndex;
  final ValueChanged<int> onDestinationSelected;
  final Widget? floatingActionButton;
  final List<Widget>? actions;

  const PremiumScaffold({
    super.key,
    required this.title,
    this.subtitle,
    required this.destinations,
    required this.bodies,
    required this.currentIndex,
    required this.onDestinationSelected,
    this.floatingActionButton,
    this.actions,
  });

  @override
  Widget build(BuildContext context) {
    final screenType = ResponsiveUtils.getScreenType(context);

    return Scaffold(
      body: Row(
        children: [
          if (screenType != ScreenType.mobile)
            _buildSideNavigation(context, screenType),
          Expanded(
            child: Column(
              children: [
                _buildAppBar(context, screenType),
                Expanded(child: bodies[currentIndex]),
              ],
            ),
          ),
        ],
      ),
      bottomNavigationBar: screenType == ScreenType.mobile
          ? _buildBottomNavigation(context)
          : null,
      floatingActionButton: floatingActionButton,
    );
  }

  Widget _buildAppBar(BuildContext context, ScreenType screenType) {
    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: ResponsiveUtils.responsiveWidth(
          context,
          mobile: 16,
          tablet: 24,
          desktop: 32,
        ),
        vertical: 16,
      ),
      decoration: BoxDecoration(
        color: AppColors.obsidian.withOpacity(0.5),
        border: Border(
          bottom: BorderSide(color: Colors.white.withOpacity(0.06), width: 1),
        ),
      ),
      child: Row(
        children: [
          if (screenType == ScreenType.mobile)
            IconButton(
              icon: const Icon(Icons.menu_rounded, size: 24),
              onPressed: () {
                showModalBottomSheet(
                  context: context,
                  backgroundColor: Colors.transparent,
                  builder: (context) => _buildMobileDrawer(context),
                );
              },
            ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: Theme.of(context).textTheme.headlineMedium),
                if (subtitle != null) ...[const SizedBox(height: 4), subtitle!],
              ],
            ),
          ),
          if (actions != null) ...actions!,
        ],
      ),
    );
  }

  Widget _buildSideNavigation(BuildContext context, ScreenType screenType) {
    final isDesktop = screenType == ScreenType.desktop;

    return Container(
      width: isDesktop ? 280 : 88,
      decoration: BoxDecoration(
        color: AppColors.obsidian,
        border: Border(
          right: BorderSide(color: Colors.white.withOpacity(0.06), width: 1),
        ),
      ),
      child: Column(
        children: [
          const SizedBox(height: 28),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    gradient: AppColors.auroraGradient,
                    borderRadius: BorderRadius.circular(14),
                    boxShadow: [
                      BoxShadow(
                        color: AppColors.primaryPink.withOpacity(0.5),
                        blurRadius: 15,
                        spreadRadius: 0,
                      ),
                    ],
                  ),
                  child: const Icon(
                    Icons.school_rounded,
                    color: Colors.white,
                    size: 24,
                  ),
                ),
                if (isDesktop) ...[
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Graduation',
                          style: Theme.of(context).textTheme.titleMedium
                              ?.copyWith(
                                color: Colors.white,
                                fontWeight: FontWeight.w700,
                              ),
                        ),
                        Text(
                          'Platform 2026',
                          style: Theme.of(context).textTheme.bodySmall
                              ?.copyWith(
                                color: AppColors.mutedText,
                                fontSize: 11,
                              ),
                        ),
                      ],
                    ),
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(height: 36),
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              itemCount: destinations.length,
              itemBuilder: (context, index) {
                final isSelected = currentIndex == index;
                final dest = destinations[index];

                return Padding(
                  padding: const EdgeInsets.only(bottom: 6),
                  child: Material(
                    color: Colors.transparent,
                    child: InkWell(
                      borderRadius: BorderRadius.circular(14),
                      onTap: () => onDestinationSelected(index),
                      child: Container(
                        padding: EdgeInsets.symmetric(
                          horizontal: isDesktop ? 16 : 14,
                          vertical: 14,
                        ),
                        decoration: BoxDecoration(
                          gradient: isSelected
                              ? AppColors.auroraGradient
                              : null,
                          color: isSelected
                              ? null
                              : Colors.white.withOpacity(0.0),
                          borderRadius: BorderRadius.circular(14),
                          border: isSelected
                              ? null
                              : Border.all(
                                  color: Colors.white.withOpacity(0.0),
                                ),
                          boxShadow: isSelected
                              ? [
                                  BoxShadow(
                                    color: AppColors.primaryPink.withOpacity(
                                      0.4,
                                    ),
                                    blurRadius: 15,
                                    offset: const Offset(0, 5),
                                  ),
                                ]
                              : null,
                        ),
                        child: Row(
                          children: [
                            Icon(
                              isSelected ? dest.selectedIcon : dest.icon,
                              color: isSelected
                                  ? Colors.white
                                  : AppColors.mutedText,
                              size: 22,
                            ),
                            if (isDesktop) ...[
                              const SizedBox(width: 14),
                              Expanded(
                                child: Text(
                                  dest.label,
                                  style: Theme.of(context).textTheme.bodyMedium
                                      ?.copyWith(
                                        color: isSelected
                                            ? Colors.white
                                            : AppColors.mutedText,
                                        fontWeight: isSelected
                                            ? FontWeight.w600
                                            : FontWeight.normal,
                                      ),
                                ),
                              ),
                            ],
                          ],
                        ),
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBottomNavigation(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.obsidian,
        border: Border(
          top: BorderSide(color: Colors.white.withOpacity(0.06), width: 1),
        ),
      ),
      child: BottomNavigationBar(
        currentIndex: currentIndex,
        onTap: onDestinationSelected,
        items: destinations.map((dest) {
          return BottomNavigationBarItem(
            icon: Icon(dest.icon),
            activeIcon: Icon(dest.selectedIcon),
            label: dest.label,
          );
        }).toList(),
      ),
    );
  }

  Widget _buildMobileDrawer(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        color: AppColors.obsidian,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(28)),
        border: Border(
          top: BorderSide(color: Colors.white.withOpacity(0.1), width: 1),
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 40,
            height: 4,
            decoration: BoxDecoration(
              color: Colors.white.withOpacity(0.2),
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(height: 24),
          ...List.generate(destinations.length, (index) {
            final isSelected = currentIndex == index;
            final dest = destinations[index];
            return ListTile(
              leading: Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  gradient: isSelected ? AppColors.auroraGradient : null,
                  color: isSelected ? null : Colors.white.withOpacity(0.05),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Icon(
                  isSelected ? dest.selectedIcon : dest.icon,
                  color: isSelected ? Colors.white : AppColors.mutedText,
                  size: 18,
                ),
              ),
              title: Text(
                dest.label,
                style: TextStyle(
                  color: isSelected ? Colors.white : AppColors.secondaryText,
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
                ),
              ),
              onTap: () {
                Navigator.pop(context);
                onDestinationSelected(index);
              },
            );
          }),
        ],
      ),
    );
  }
}
