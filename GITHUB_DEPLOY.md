# 🚀 دليل النشر على GitHub — NullGate WAF

## الخطوة 1: إنشاء Repository على GitHub

1. اذهب إلى https://github.com/new
2. **Repository name**: `NullGate`
3. **Description**: `Enterprise-Grade Web Application Firewall — Fail-closed, multi-layer defense with 64 detection rules`
4. اختر **Public** (حتى تستطيع الشركات رؤيته)
5. **لا تختار** "Initialize with README" (لأنه موجود بالفعل)
6. اضغط **Create repository**

---

## الخطوة 2: ربط المشروع المحلي بـ GitHub

افتح terminal/PowerShell في مجلد المشروع وأكتب:

```bash
# استبدل USERNAME باسم حسابك على GitHub
git remote add origin https://github.com/fatim1amer-netizen/NullGate.git

# تأكد من الربط
git remote -v
```

---

## الخطوة 3: رفع الكود

```bash
# Push إلى main branch
git push -u origin master

# إذا طلب منك اسم المستخدم وكلمة المرور:
# Username: fatim1amer-netizen
# Password: استخدم Personal Access Token (ليس كلمة المرور العادية)
```

### ✅ إنشاء Personal Access Token (إذا لزم الأمر):

1. اذهب إلى https://github.com/settings/tokens
2. اضغط **Generate new token (classic)**
3. أعطه اسم: `NullGate Push`
4. اختر صلاحيات: `repo` (كل الصلاحيات تحت repo)
5. اضغط **Generate token**
6. **انسخ الـ token فورًا** (لن تستطيع رؤيته مرة أخرى)
7. استخدمه بدلاً من كلمة المرور عند الـ push

---

## الخطوة 4: التحقق من النشر

بعد الـ push، اذهب إلى:
```
https://github.com/fatim1amer-netizen/NullGate
```

ستجد:
- ✅ كل الكود
- ✅ README.md يظهر مباشرة (تعريف المشروع)
- ✅ GitHub Actions CI يبدأ التشغيل تلقائيًا
- ✅ الـ landing page HTML
- ✅ سكريبتات العرض التجريبي

---

## 🎯 كيف تشارك المشروع مع الشركات

### الرابط الرئيسي:
```
https://github.com/fatim1amer-netizen/NullGate
```

### طرق الاستخدام للشركات:

#### 1️⃣ **Clone المشروع وتشغيله محليًا** (للاختبار):
```bash
git clone https://github.com/fatim1amer-netizen/NullGate.git
cd NullGate
go build -o nullgate cmd/nullgate/main.go
./nullgate --config configs/config.yaml
```

#### 2️⃣ **نشر على VPS خاص بهم** (للإنتاج):
```bash
# على السيرفر:
git clone https://github.com/fatim1amer-netizen/NullGate.git
cd NullGate
go build -o nullgate cmd/nullgate/main.go

# تشغيل مع HTTPS تلقائي:
./nullgate --config configs/config-cloud.yaml \
           --auto-tls \
           --domain their-company.com
```

#### 3️⃣ **Docker** (الأسهل):
```bash
git clone https://github.com/fatim1amer-netizen/NullGate.git
cd NullGate
docker compose up --build
```

---

## 📊 ميزات للشركات

### ✨ **صفحة Landing احترافية**:
عند تفعيل `serve_landing: true` في الكونفيج، الشركة تفتح `https://their-domain.com` وتظهر صفحة تسويقية للـ WAF بنفس الدومين.

### 🎭 **عرض تجريبي حي**:
الشركات تستطيع تشغيل العرض التجريبي المباشر:
```bash
# Windows:
scripts\live-demo.cmd

# Linux/Mac:
bash scripts/live-demo.sh
```
هذا يطلق 17 عينة اختبار هجومية؛ النتائج تصف هذه العينات فقط ولا تمثل نسبة كشف ميدانية.

### 🔐 **لوحة تحكم محمية**:
الشركات تصل للوحة التحكم عبر SSH tunnel:
```bash
ssh -L 9091:127.0.0.1:9091 user@their-server
# ثم افتح: http://localhost:9091
```

### 📦 **نشر جاهز**:
ملف `deploy/DEPLOY.md` يحتوي خطوات كاملة للنشر على:
- DigitalOcean
- AWS
- Hetzner
- Vultr
- أي VPS

---

## 🎓 لبحث التخرج

### نقاط القوة للعرض على اللجنة:

1. **مشروع مفتوح المصدر احترافي** على GitHub
2. **CI/CD** جاهز (GitHub Actions)
3. **Docker** للنشر السهل
4. نجاح الحالات الموسومة في corpus الاختبار (ليس ضمانًا لمعدل الكشف في الإنتاج)
5. **عرض تجريبي حي** يمكن تشغيله أمام اللجنة
6. **توثيق كامل** (README + ARCHITECTURE + DEPLOY)
7. اختبارات آلية للمكونات وتكامل مسار الطلب
8. **Let's Encrypt** للـ HTTPS التلقائي

---

## 📞 للتواصل مع الشركات

أرسل لهم:
```
مرحبًا،

طورت NullGate WAF — جدار حماية تطبيقات ويب متعدد الطبقات:
- 64 قاعدة كشف عبر 12 فئة هجوم
- نجاح عينات محددة في الاختبارات، دون ادعاء معدل ميداني
- نشر بأمر واحد مع HTTPS تلقائي
- لوحة تحكم حية محمية
- مفتوح المصدر بالكامل

الكود: https://github.com/fatim1amer-netizen/NullGate
العرض التجريبي: متاح (17 عينة هجومية؛ النتائج مرتبطة بإعداد العرض)

هل تهتمون باختباره في بيئتكم؟

مع التحية،
[اسمك]
```

---

## ⚡ الخطوة التالية (اختياري)

بعد النشر، يمكنك:

1. **إضافة Badge للـ README**:
   - Build status من GitHub Actions
   - Go version
   - License

2. **إنشاء Releases**:
```bash
git tag -a v2.0.0 -m "NullGate WAF v2.0.0 - Initial Release"
git push origin v2.0.0
```
ثم اذهب لـ GitHub → Releases → Create Release

3. **GitHub Pages** (اختياري):
   نشر صفحة الـ landing على `https://fatim1amer-netizen.github.io/NullGate`

---

**الآن جاهز للنشر! 🚀**
