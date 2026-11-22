# 🌐 NullGate WAF - Public Deployment Guide

هذا الدليل يشرح كيفية تشغيل NullGate كخدمة عامة يمكن الوصول إليها من أي مكان في العالم.

---

## ⚡ النشر السريع (Quick Deploy)

قبل النشر، اضبط `server.target_backend` في `configs/config.yaml` ليشير إلى
تطبيقك الموثوق، واجعل عنوانه قابلاً للوصول من NullGate. لا تستخدم التطبيق
الضعيف الموجود في مجلد `examples` في نشر عام. ملف Compose العام يشغّل WAF
والنفق فقط، ولا يشغّل backend تجريبياً.

### الطريقة 1: سكربت تلقائي (أسهل طريقة)

**Windows:**
```cmd
deploy-public.cmd
```

**Linux/macOS:**
```bash
chmod +x deploy-public.sh
./deploy-public.sh
```

بعد التشغيل ستحصل على رابط عام مثل:
```
https://random-name-1234.trycloudflare.com
```

### تشغيل موقع المشروع الأصلي ومختبر الأمن محلياً فقط

لعرض المشروع والصفحة الأصلية مع مختبر تفاعلي وقاعدة SQLite، استخدم `START-SHOWCASE.cmd` من مجلد المشروع. يتطلب Go 1.25 أو أحدث، ولا يشغّل Cloudflare Tunnel أو أي backend مكشوف:

```cmd
START-SHOWCASE.cmd
```

افتح `http://127.0.0.1:8080/original/` للموقع الأصلي و`http://127.0.0.1:8080/cyberlab/` للمختبر. هذه الروابط لا تحتاج إعداد DNS. ويمكن استخدام `http://nullgate.localhost:8080/` و`http://cyberlab.localhost:8080/` عند نجاح حل `.localhost`. إذا لم تعمل أسماء النطاقات، استخدم روابط `127.0.0.1` أعلاه أو أضف السطر `127.0.0.1 nullgate.localhost cyberlab.localhost` إلى ملف hosts. بيانات الحسابات والحجوزات محفوظة محلياً في `data/showcase.db`. كلمة مرور لوحة الإدارة المحلية تظهر مرة واحدة في نافذة التشغيل عند الإعداد الأول. هذا الوضع مخصص للعرض على الجهاز نفسه؛ لا تربطه بنفق عام.

### الطريقة 2: Docker Compose (الأفضل للإنتاج)

```bash
# تشغيل WAF والنفق (اضبط upstream آمن في config.yaml أولاً)
docker-compose -f docker-compose-public.yml up -d

# عرض الرابط العام
docker-compose -f docker-compose-public.yml logs tunnel

# سترى شيئاً مثل:
# https://random-name-5678.trycloudflare.com
```

---

## 🔐 الأمان (IMPORTANT!)

لا توجد كلمة مرور افتراضية. النفق العام يمرر إلى مستمع NullGate المحمي فقط؛ لوحة الإدارة والخادم الخلفي لا يُنشران عبر النفق. لتفعيل الإدارة، عيّن كلمة مرور قوية وأبقِ المستمع على loopback:

```bash
# عدّل configs/config.yaml
admin:
  enabled: true
  listen_addr: "127.0.0.1:9091"
  password_hash: "sha256:your-hash-here"
  read_only: false
  require_localhost: true
```

### تفعيل HTTPS (اختياري)

Cloudflare Tunnel يوفر HTTPS تلقائياً بدون شهادات.

---

## 🛠 المتطلبات

### الخيار A: تثبيت Cloudflare Tunnel (مجاني - مُوصى به)

**Windows:**
1. تحميل من: https://github.com/cloudflare/cloudflared/releases
2. وضع `cloudflared.exe` في مجلد المشروع أو في PATH

**Linux:**
```bash
wget https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64
chmod +x cloudflared-linux-amd64
sudo mv cloudflared-linux-amd64 /usr/local/bin/cloudflared
```

**macOS:**
```bash
brew install cloudflared
```

### الخيار B: تثبيت ngrok (بديل)

تحميل من: https://ngrok.com/download

---

## 📱 الاستخدام

### 1. تشغيل المشروع

```bash
# Windows
deploy-public.cmd

# Linux/macOS
./deploy-public.sh

# أو Docker
docker-compose -f docker-compose-public.yml up -d
```

### 2. الحصول على الرابط العام

السكربت سيعرض الرابط تلقائياً، مثل:
```
============================================
  YOUR PUBLIC URL IS SHOWN ABOVE
  Look for: https://xxxxx.trycloudflare.com
============================================
```

### 3. مشاركة الرابط

أرسل الرابط لأي شخص. يمكنهم فتحه من:
- أي جهاز كمبيوتر
- أي هاتف
- أي مكان في العالم
- بدون VPN أو إعدادات

### 4. الاستخدام الآمن

الرابط العام يعرض التطبيق المار عبر WAF فقط. لوحة الإدارة تبقى محلية على `http://127.0.0.1:9091` عند تفعيلها، وإدارة التطبيقات تتطلب جلسة موثقة و`read_only: false`.

---

## 🎯 ميزات النشر العام

✅ **رابط واحد فقط** - لا حاجة لـ localhost
✅ **HTTPS مجاني** - Cloudflare توفره تلقائياً
✅ **لا حاجة لـ VPS** - يعمل من جهازك
✅ **لا إعدادات شبكة** - لا port forwarding أو firewall
✅ **حماية WAF مركزية** - يمر التطبيق عبر NullGate
✅ **عدم نشر backend مباشرة** - النفق يصل إلى مستمع WAF فقط
✅ **إدارة محلية اختيارية** - لا توجد كلمة مرور افتراضية، ولا يُنشر منفذ الإدارة

---

## 🔧 إعدادات متقدمة

### تخصيص المنفذ (Port)

في `configs/config.yaml`:

```yaml
admin:
  listen_addr: "127.0.0.1:9091"  # لا تنشر لوحة الإدارة عبر النفق العام
```

### تشغيل بدون demo backend

```bash
# تشغيل WAF فقط
./nullgate --config configs/config.yaml --admin

# النفق العام يذهب إلى WAF فقط، لا إلى لوحة الإدارة
cloudflared tunnel --url http://localhost:8080
```

### استخدام domain خاص (Cloudflare فقط)

1. سجل دخول لـ Cloudflare
2. أنشئ tunnel دائم:
```bash
cloudflared tunnel login
cloudflared tunnel create nullgate
cloudflared tunnel route dns nullgate waf.yourdomain.com
```

---

## 🐳 Docker Commands مفيدة

```bash
# عرض اللوقات المباشرة
docker-compose -f docker-compose-public.yml logs -f

# عرض الرابط العام فقط
docker-compose -f docker-compose-public.yml logs tunnel | grep "https://"

# إيقاف كل شيء
docker-compose -f docker-compose-public.yml down

# إعادة البناء
docker-compose -f docker-compose-public.yml up --build -d

# عرض حالة الخدمات
docker-compose -f docker-compose-public.yml ps
```

---

## ❓ حل المشاكل (Troubleshooting)

### المشكلة: "No tunnel service found"

**الحل:** ثبت cloudflared أو ngrok (انظر المتطلبات أعلاه)

### المشكلة: "Build failed"

**الحل:** تأكد من تثبيت Go 1.25+
```bash
go version
# يجب أن يظهر: go version go1.25 أو أعلى
```

### المشكلة: الرابط لا يعمل

**الحل:**
1. تأكد من تشغيل الخدمات: `docker-compose ps`
2. اعرض اللوقات: `docker-compose logs -f`
3. تأكد من أن NullGate يعمل وأن `server.target_backend` يشير إلى تطبيقك الموثوق.

### المشكلة: "unauthorized" عند الدخول

**الحل:**
الرابط العام مخصص للتطبيق المحمي، وليس لوحة الإدارة. افتح لوحة الإدارة محلياً على `http://127.0.0.1:9091` بعد تفعيلها وتعيين `admin.password_hash`. لا تمرر منفذ الإدارة عبر النفق العام.

---

## 📊 التطبيق ولوحة الإدارة

الرابط العام يعرض التطبيق الذي يمر عبر NullGate. لوحة الإدارة منفصلة وتبقى على loopback؛ تتطلب إدارة التطبيقات جلسة موثقة و`admin.read_only: false`. من اللوحة يمكن عرض التطبيقات المحمية وقياساتها وسجلاتها، وتعديل السياسات والقواعد المخصصة. لا تفتح اللوحة عبر النفق العام.

---

## 🚀 خطوات ما بعد النشر

### 1. فعّل الإدارة اختيارياً وبشكل آمن
```bash
printf '%s' 'your-strong-password' | sha256sum
# ضع القيمة sha256:<hex> في configs/config.yaml.
# فعّل admin.enabled، وأبقِ listen_addr على 127.0.0.1:9091.
```

### 2. اختر وضع الإدارة
```yaml
admin:
  read_only: true  # يمنع التعديلات؛ اضبط false لإدارة التطبيقات
```

### 3. أضف backend موثوقاً
```yaml
server:
  target_backend: "http://your-real-app:3000"
applications:
  allowed_upstream_hosts:
    - "your-real-app:3000"
```

### 4. راقب اللوقات
```bash
# Docker
docker-compose -f docker-compose-public.yml logs -f waf

# مباشرة
tail -f logs/waf.log
```

---

## 🎓 أمثلة استخدام

### سيناريو 1: عرض مشروع لعميل

```bash
# شغّل المشروع
./deploy-public.sh

# احصل على الرابط
# https://abc-123.trycloudflare.com

# أرسل رابط التطبيق المحمي، وليس لوحة الإدارة.
```

### سيناريو 2: ورشة عمل أو تدريب

```bash
# شغّل بـ Docker للاستقرار
docker-compose -f docker-compose-public.yml up -d

# شارك الرابط مع المتدربين
# يمكنهم الوصول من هواتفهم مباشرة
```

### سيناريو 3: اختبار من عدة مواقع

```bash
# الرابط يعمل من أي مكان:
# - اختبره من WiFi المنزل
# - اختبره من بيانات الموبايل
# - اشرك صديق من مدينة أخرى
```

---

## 📞 الدعم

إذا واجهت أي مشكلة:

1. راجع هذا الملف أولاً
2. اعرض `README.md` الأصلي للتوثيق الكامل
3. راجع `docs/ARCHITECTURE.md` للتفاصيل التقنية

---

## ⚠️ تحذيرات الأمان

1. لا توجد كلمة مرور افتراضية؛ لا تفعّل الإدارة القابلة للكتابة دون كلمة مرور.
2. لا تنشر منفذ لوحة الإدارة أو أي backend تجريبي/ضعيف.
3. استخدم `read_only: true` للمراقبة فقط.
4. راجع اللوقات بانتظام لرصد أي نشاط مشبوه.
5. أوقف الخدمة عندما لا تحتاجها.

---

## 📝 الرابط النهائي

بعد التشغيل، ستحصل على رابط مثل:

```
🌐 https://random-name-1234.trycloudflare.com
```

**هذا هو الرابط الذي تشاركه مع أي شخص!**

- ✅ يعمل من أي مكان
- ✅ يعمل على أي جهاز
- ✅ HTTPS آمن
- ✅ لا حاجة لإعدادات
- ✅ مجاني تماماً

---

**استمتع بـ NullGate WAF! 🛡️🔥**
