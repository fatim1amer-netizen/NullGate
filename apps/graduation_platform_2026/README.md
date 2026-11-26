# Graduation Platform 2026

The original Flutter graduation-project management application from
[`fatim1amer-netizen/graduation_platform_2026`](https://github.com/fatim1amer-netizen/graduation_platform_2026).
It is included in NullGate as the protected primary application; the separate
Cybersecurity Attack Lab remains available for WAF request demonstrations.

The app's current persistence is browser-local via `shared_preferences`; it
does not have a server-side API. NullGate therefore protects delivery of the
web application, while application state changes stay in the user's browser.
Use the Cybersecurity Attack Lab to demonstrate request inspection and
blocking against actual HTTP endpoints.

Embedded student identities have been replaced with fictional demo records.
For local development:

```cmd
flutter pub get
flutter run -d chrome
```

To rebuild the static bundle embedded by NullGate:

```cmd
flutter build web --release --base-href /graduation/
```

The bundled source was copied from the upstream application's `main` branch.
