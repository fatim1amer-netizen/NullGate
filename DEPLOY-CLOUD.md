# 🚀 NullGate WAF - One-Click Cloud Deployment

هذا الدليل لنشر NullGate على الإنترنت العام **بزر واحد**.

---

## ⚡ النشر الفوري (اختر واحدة)

### الخيار 1: Render (الأسهل - مجاني)

[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/fat1322im-hub/NullGate)

**الخطوات:**
1. اضغط الزر أعلاه
2. سجل دخول/أنشئ حساب Render (مجاني)
3. اضغط **"Create Web Service"**
4. انتظر 2-3 دقائق للبناء

**ستحصل على رابط:**
```
https://nullgate-waf-xxxx.onrender.com
```

### الخيار 2: Railway (سريع - مجاني $5 شهرياً)

[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/template/nullgate?referralCode=bonus)

**أو يدوياً:**
```bash
# ثبت Railway CLI
npm i -g @railway/cli

# سجل دخول
railway login

# انشر
cd C:/Users/venom/Desktop/NullGate
railway up
railway open
```

**الرابط:**
```
https://nullgate-waf-production.up.railway.app
```

### الخيار 3: Fly.io (عالمي - مجاني)

```bash
# ثبت Fly CLI (Windows)
powershell -Command "iwr https://fly.io/install.ps1 -useb | iex"

# سجل دخول
fly auth login

# انشر (مرة واحدة)
cd C:/Users/venom/Desktop/NullGate
fly launch --yes
fly deploy

# اعرض الرابط
fly status
```

**الرابط:**
```
https://nullgate-waf.fly.dev
```

---

## 🔐 تسجيل الدخول

عند فتح أي رابط أعلاه:

```
Username: admin
Password: [generated automatically - check logs]
```

**للحصول على كلمة المرور:**

**Render:**
```
Dashboard → Environment → ADMIN_PASSWORD
```

**Railway:**
```bash
railway logs
# ابحث عن: "Generated admin password: ..."
```

**Fly.io:**
```bash
fly secrets list
```

**تغيير كلمة المرور:**

```bash
# Render/Railway
ADMIN_PASSWORD=your-new-password

# Fly.io
fly secrets set ADMIN_PASSWORD=your-new-password
```

---

## ✅ ماذا حدث تقنياً

تم تعديل المشروع ليعمل على السحابة:

1. **`cmd/nullgate/main.go`:**
   - يقرأ `$PORT` تلقائياً (المنصات تحقنه)
   - يربط لوحة التحكم بهذا المنفذ مباشرة
   - يفعّل `--admin` تلقائياً في بيئة السحابة

2. **ملفات النشر:**
   - `render.yaml` - Render config
   - `railway.json` - Railway config  
   - `fly.toml` - Fly.io config
   - `Dockerfile` - محدّث لـ `EXPOSE $PORT`

3. **الأمان:**
   - كلمة مرور مولّدة تلقائياً
   - HTTPS مجاني من المنصة
   - `X-Forwarded-For` موثوق للحصول على IP الحقيقي

---

## 🌐 الرابط النهائي

بعد النشر، ستحصل على رابط مثل:

```
https://nullgate-waf-abc123.onrender.com
```

**شاركه مع أي شخص - يعمل فوراً!**

---

## 🔧 إعدادات متقدمة

### تغيير الـ Backend

افتراضياً، الـ WAF يحمي `https://jsonplaceholder.typicode.com` (للتجربة).

**لتوجيهه لتطبيقك:**

```bash
# Render
Environment → TARGET_BACKEND → https://your-app.com

# Railway
railway variables set TARGET_BACKEND=https://your-app.com

# Fly.io
fly secrets set TARGET_BACKEND=https://your-app.com
```

### عرض اللوقات المباشرة

```bash
# Render
Dashboard → Logs

# Railway
railway logs -f

# Fly.io
fly logs
```

### التحديث (Deploy جديد)

```bash
# دفع التغييرات إلى GitHub
git add .
git commit -m "update"
git push

# Render/Railway: يُحدّث تلقائياً
# Fly.io: fly deploy
```

---

## ❓ استكشاف الأخطاء

### المشكلة: "Application failed to respond"

**الحل:**
```bash
# اعرض اللوقات
railway logs  # أو fly logs

# تأكد من تشغيل PORT
railway variables  # يجب أن يكون PORT موجوداً
```

### المشكلة: "unauthorized" عند الدخول

**الحل:**
```bash
# احصل على كلمة المرور من Environment Variables
railway variables | grep ADMIN_PASSWORD
```

### المشكلة: البناء فشل

**الحل:**
```bash
# تأكد من Go version في go.mod
# يجب: go 1.22 أو أحدث
```

---

## 📊 التكلفة

| المنصة | المجاني | الحدود |
|---|---|---|
| **Render** | ✅ نعم | 750 ساعة/شهر، ينام بعد 15 دقيقة |
| **Railway** | ✅ $5 رصيد | 500 ساعة/شهر، لا sleep |
| **Fly.io** | ✅ نعم | 3 VMs صغيرة، لا sleep |

**التوصية:** ابدأ بـ **Render** (أسهل)، ثم انتقل لـ **Fly.io** (أفضل أداء).

---

## 🎉 جاهز!

الآن المشروع منشور على الإنترنت بالكامل. شارك الرابط مع أي شخص!

**الخطوات التالية:**
1. انشر الآن باستخدام أحد الأزرار أعلاه
2. احصل على الرابط من Dashboard
3. شاركه مع الآخرين
4. سجّل دخول بـ admin + password من Environment Variables

---

**Need the actual link NOW?** Follow Option 1 (Render) - it's literally 3 clicks.
