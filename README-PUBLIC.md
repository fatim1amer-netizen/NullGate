# 🌐 NullGate WAF - Public Deployment Guide

هذا الدليل يشرح كيفية تشغيل NullGate كخدمة عامة يمكن الوصول إليها من أي مكان في العالم.

---

## ⚡ النشر السريع (Quick Deploy)

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

### الطريقة 2: Docker Compose (الأفضل للإنتاج)

```bash
# تشغيل كل شيء (WAF + Backend + Tunnel)
docker-compose -f docker-compose-public.yml up -d

# عرض الرابط العام
docker-compose -f docker-compose-public.yml logs tunnel

# سترى شيئاً مثل:
# https://random-name-5678.trycloudflare.com
```

---

## 🔐 الأمان (IMPORTANT!)

### كلمة المرور الافتراضية

```
Username: admin
Password: admin123
```

**⚠️ يجب تغيير كلمة المرور فوراً!**

لتغيير كلمة المرور:

```bash
# 1. إنشاء hash جديد
echo -n "your-new-password" | sha256sum

# 2. تعديل configs/config.yaml
admin:
  password_hash: "sha256:your-hash-here"
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

### 4. تسجيل الدخول

عند فتح الرابط:
1. Username: `admin`
2. Password: `admin123` (غيّرها فوراً!)
3. ستظهر لوحة التحكم الكاملة

---

## 🎯 ميزات النشر العام

✅ **رابط واحد فقط** - لا حاجة لـ localhost  
✅ **HTTPS مجاني** - Cloudflare توفره تلقائياً  
✅ **لا حاجة لـ VPS** - يعمل من جهازك  
✅ **لا إعدادات شبكة** - لا port forwarding أو firewall  
✅ **متعدد اللغات** - واجهة عربية/إنجليزية  
✅ **مصادقة آمنة** - SHA-256 + sessions  
✅ **متوافق مع الموبايل** - responsive design  

---

## 🔧 إعدادات متقدمة

### تخصيص المنفذ (Port)

في `configs/config.yaml`:

```yaml
admin:
  listen_addr: "0.0.0.0:9091"  # غيّر 9091 لأي منفذ آخر
```

### تشغيل بدون demo backend

```bash
# تشغيل WAF فقط
./nullgate --config configs/config.yaml --admin

# ثم tunnel
cloudflared tunnel --url http://localhost:9091
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

**الحل:** تأكد من تثبيت Go 1.21+
```bash
go version
# يجب أن يظهر: go version go1.21 أو أعلى
```

### المشكلة: الرابط لا يعمل

**الحل:**
1. تأكد من تشغيل الخدمات: `docker-compose ps`
2. اعرض اللوقات: `docker-compose logs -f`
3. تأكد من admin.enabled: true في config.yaml

### المشكلة: "unauthorized" عند الدخول

**الحل:**
1. تأكد من كلمة المرور الصحيحة
2. امسح الكوكيز وحاول مجدداً
3. راجع `configs/config.yaml` للـ password_hash

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
# "Password: admin123"
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

1. **غيّر كلمة المرور الافتراضية فوراً**
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
