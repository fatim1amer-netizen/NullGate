import 'package:flutter/material.dart';
import 'app_theme.dart';
import 'security_manager.dart';
import 'storage_service.dart';
import 'data_models.dart';
import 'screens.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await StorageService.init();
  await SecurityManager.initialize();
  await AppState.loadData();
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'منصة إدارة مشاريع التخرج 2026',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.cyberTheme,
      builder: (context, child) {
        return Directionality(textDirection: TextDirection.rtl, child: child!);
      },
      home: const SplashScreen(),
    );
  }
}
