# 🌐 NullGate WAF - Public Access Guide

هذا الدليل يشرح طريقتين مختلفتين: نفقًا مؤقتًا للتجربة من جهازك، ونشرًا سحابيًا
مستقلًا عن جهازك. لا تستخدم النفق المؤقت كاستضافة إنتاجية.

---

## ⚡ النشر السريع (Quick Deploy)

### الطريقة 1: نفق مؤقت للتجربة (يتطلب بقاء جهازك مشغّلًا)

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

### الطريقة 2: Docker Compose مع نفق مؤقت

```bash
# تشغيل WAF أمام موقع التخرج الحقيقي عبر نفق عام
docker-compose -f docker-compose-public.yml up -d

# عرض الرابط العام
docker-compose -f docker-compose-public.yml logs tunnel

# سترى شيئاً مثل:
# https://random-name-5678.trycloudflare.com
```

هذه الطريقة ما زالت تعتمد على Docker و`cloudflared` يعملان على جهازك. للنشر
المستقل على الإنترنت استخدم إعدادات Render أو Fly.io الموضحة في
[`DEPLOY-CLOUD.md`](DEPLOY-CLOUD.md).

---

## 🔐 الأمان

الرابط العام يمر عبر منفذ البروكسي `8080` ولا يعرّض لوحة التحكم. سكربتات
النشر تفعّل لوحة التحكم محليًا على `127.0.0.1:9091`؛ ملف Docker Compose العام
يتركها معطّلة. لا توجد كلمة مرور افتراضية أو بيانات دخول عامة. لا تنشر لوحة
التحكم على الإنترنت؛ استخدم نفق SSH للوصول البعيد.

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

### 4. الموقع خلف WAF

سكربتات النشر العامة وملف Docker Compose يستخدمون ملف الإعداد الذي يمرّر الموقع
الحقيقي إلى Cloudflare Pages:

```bash
go run ./cmd/nullgate --config configs/graduation-platform.yaml
```

ثم وجّه اسم نطاق تملكه إلى NullGate. الـ backend الضعيف موجود للاختبارات المحلية
فقط، عبر `docker-compose.yml` أو `scripts/quickstart.sh`.

---

## 🎯 خصائص النفق المؤقت

✅ **رابط واحد فقط** - لا حاجة لـ localhost  
✅ **HTTPS مجاني** - Cloudflare توفره تلقائياً  
⚠️ **يعتمد على جهازك** - يتوقف الرابط عند إيقاف NullGate أو النفق أو الجهاز
✅ **لا إعدادات شبكة** - لا port forwarding أو firewall  
✅ **التطبيق خلف WAF** - الرابط المؤقت يمر عبر NullGate إلى backend المحدد
✅ **لوحة التحكم منفصلة** - محلية افتراضيًا؛ لا تُعرَض عبر النفق العام
✅ **لا بيانات دخول افتراضية** - لا ترسل رابط النفق إلى لوحة الإدارة

---

## 🔧 إعدادات متقدمة

### تخصيص المنفذ (Port)

في ملف الإعداد الذي تستخدمه:

```yaml
admin:
  listen_addr: "127.0.0.1:9091"  # اتركه محليًا واستخدم SSH للوصول البعيد
```

### تشغيل الموقع الحقيقي يدويًا

```bash
# تشغيل WAF فقط
./nullgate --config configs/graduation-platform.yaml --admin

# ثم tunnel
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
# يجب أن يظهر: go version go1.21 أو أعلى
```

### المشكلة: الرابط لا يعمل

**الحل:**
1. تأكد من تشغيل الخدمات: `docker-compose ps`
2. اعرض اللوقات: `docker-compose logs -f`
3. فعّل `admin.enabled` فقط عند الحاجة، واترك `require_localhost: true`.

### المشكلة: "unauthorized" عند الدخول

**الحل:**
1. تأكد من كلمة المرور الصحيحة
2. امسح الكوكيز وحاول مجدداً
3. راجع ملف الإعداد المستخدم لقيمة `password_hash` إذا كنت قد فعّلت المصادقة

---

## 📊 لوحة التحكم

عند فتح الرابط العام سترى:

### 📈 KPIs
- إجمالي الطلبات
- الطلبات المحظورة
- معدل الحظر
- زمن الاستجابة
- وقت التشغيل

### 🔥 المميزات
- **رسم بياني مباشر** للترافيك (يتحدث كل ثانيتين)
- **أنواع الهجمات** (SQLi, XSS, RCE...)
- **طبقات الدفاع** (11 طبقة حماية)
- **الأحداث الأمنية** (100 حدث أخير)
- **التحكم المباشر** (حظر/فك حظر IPs)
- **مسح الحماية** (اختبار موقعك الخاص)

### 🌐 واجهة مزدوجة
- تبديل فوري: English ↔ العربية
- دعم RTL كامل
- Dark/Light mode
- Responsive (موبايل + تابلت + ديسكتوب)

---

## 🚀 خطوات ما بعد النشر

### 1. غيّر كلمة المرور فوراً
```bash
echo -n "new-password" | sha256sum
# ضع الناتج في configs/config.yaml
```

### 2. فعّل read_only إذا كنت تريد مراقبة فقط
```yaml
admin:
  read_only: true  # يمنع التعديلات من اللوحة
```

### 3. أضف backend حقيقي
```yaml
server:
  target_backend: "http://your-real-app:3000"
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

# أرسل للعميل:
# "تفضل رابط لوحة التحكم: https://abc-123.trycloudflare.com"
# "Username: admin"
# لا توجد بيانات دخول افتراضية؛ لوحة التحكم محلية فقط افتراضيًا
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

1. **لا تعرض لوحة التحكم على الإنترنت**
2. **لا تترك demo backend مكشوفاً** (المنفذ 3000 يجب ألا يكون عاماً)
3. **استخدم read_only: true** للمراقبة فقط
4. **راجع اللوقات بانتظام** لرصد أي نشاط مشبوه
5. **أوقف الخدمة** عندما لا تحتاجها

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
