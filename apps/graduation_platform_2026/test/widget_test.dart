import 'package:flutter_test/flutter_test.dart';

import 'package:graduation_platform_2026/main.dart';

void main() {
  testWidgets('graduation platform splash screen renders', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const MyApp());
    expect(find.text('GRADUATION PLATFORM 2026'), findsOneWidget);
    await tester.pump(const Duration(seconds: 6));
  });
}
