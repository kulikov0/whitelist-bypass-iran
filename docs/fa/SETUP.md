# شروع سریع

فایل‌های مورد نیاز را از [گیت‌هاب ریلیز](https://github.com/kulikov0/whitelist-bypass-iran/releases) دانلود کنید.

> English: [docs/en/SETUP.md](../en/SETUP.md)

## فهرست

- [پیش‌نیازها](#پیش-نیازها)
- [Creator (دسکتاپ)](#creator-دسکتاپ)
- [Creator (هدلس، سرور)](#creator-هدلس-سرور)
- [Joiner (اندروید)](#joiner-اندروید)
- [Joiner (آیفون)](#joiner-آیفون)
- [Joiner (دسکتاپ: ویندوز / مک / لینوکس)](#joiner-دسکتاپ-ویندوز--مک--لینوکس)
- [Joiner (لینوکس، هدلس)](#joiner-لینوکس-هدلس)

## پیش‌نیازها

- **Creator** (سمت اینترنت آزاد) - دسکتاپ یا هدلس روی سرور
- **Joiner** (سمت سانسور) - اندروید، آیفون یا دسکتاپ

دو حالت تونل: **Video** (VP8) و **DC** (DataChannel). Creator هدلس به‌صورت خودکار با حالتی که Joiner انتخاب می‌کند هماهنگ می‌شود، در هر اتصال جدید. حالت Video پیش‌فرض است؛ DC پهنای باند کمتری دارد ولی CPU را کمی کمتر مصرف می‌کند.

> Creator باید روی اینترنتی بدون محدودیت اجرا شود (خارج از ایران یا روی یک VPN کارا) و قبل از هر تلاش Joiner برای اتصال باید آنلاین باشد. هم Creator و هم Joiner پشت پرده هدلس هستند - Pion مستقیماً با SFU بله صحبت می‌کند، هیچ مرورگری در کار نیست.

## Creator (دسکتاپ)

![رابط](../res/desktop_interface.png)

1. اپلیکیشن Creator را دانلود و اجرا کنید
2. روی **+** کلیک کنید تا یک تب جدید باز شود
3. روی **Bale** کلیک کنید
4. در پنجره داخلی به حساب بله خود وارد شوید
5. اپ به‌صورت خودکار یک تماس می‌سازد و لینک اتصال را نمایش می‌دهد
6. لینک اتصال را کپی کرده و برای Joiner بفرستید

برای اجرای Creator بدون رابط گرافیکی (روی سرور)، کوکی‌ها را با دکمه **Bale Cookies** خارج کنید و با باینری هدلس استفاده کنید (پایین را ببینید).

> اجرای Creator دسکتاپ روی VPS بدون محیط گرافیکی از طریق XPRA - راهنمای جداگانه: [VPS.md](VPS.md).

## Creator (هدلس، سرور)

برای اجرای Creator روی سرور بدون رابط گرافیکی.

### آماده‌سازی کوکی‌ها

برای احراز هویت در بله کوکی لازم است. آن‌ها را از Creator دسکتاپ خارج کنید:

1. Creator را روی یک دستگاه دسکتاپ باز کنید
2. به حساب بله وارد شوید (حالت عادی)
3. روی **Bale Cookies** کلیک کنید
4. فایل `bale-cookies.json` ایجادشده را به سرور خود کپی کنید

### اجرا

```sh
./headless-bale-creator --cookies bale-cookies.json
```

Creator یک تماس می‌سازد و لینک اتصال را در لاگ چاپ می‌کند. آن لینک را به Joiner بفرستید.

اگر `bale-cookies.json` گم شده یا منقضی شده باشد، باینری `WAITING_FOR_COOKIES` چاپ می‌کند و یک رشته کوکی تازه را از stdin می‌خواند. برای خط‌لوله‌های re-auth خودکار مفید است.

### پرچم‌ها

| پرچم | توضیحات |
|---|---|
| `--cookies <path>` | مسیر فایل JSON کوکی‌ها |
| `--cookie-string <str>` | کوکی به‌صورت رشته خام (`name=val; name=val`) |
| `--resources <mode>` | `default` / `moderate` / `unlimited` |
| `--write-file <path>` | لینک اتصال فعال در این فایل اضافه می‌شود (یک خط در هر نشست) |
| `--vp8-fps <n>` | نرخ فریم VP8 (پیش‌فرض `24`) |
| `--vp8-batch <n>` | ضریب batch فریم VP8 (پیش‌فرض `30`) |

### حالت‌های منابع

| حالت | `read-buf` | `mem-limit` | کاربرد |
|---|---|---|---|
| `moderate`  | 16 KB | 64 MB  | VPS با رم کم |
| `default`   | 32 KB | 128 MB | استفاده عادی |
| `unlimited` | 64 KB | 256 MB | حداکثر throughput (ممکن است به‌خاطر congestion control کند شود) |

- `read-buf` - اندازه بافر خواندن TCP. کوچک‌تر = چک‌های backpressure بیشتر، مصرف رم منظم‌تر.
- `mem-limit` - حد نرم حافظه ران‌تایم Go؛ نزدیک سقف GC تهاجمی‌تر می‌شود.

## Joiner (اندروید)

![رابط](../res/android_interface.png)

1. فایل `whitelist-bypass.apk` را دانلود و نصب کنید
2. در اولین اجرا، اجازه VPN را در دیالوگ سیستم بدهید
3. تنظیمات را باز کنید (آیکن سمت راست GO) و حالت تونل را انتخاب کنید (**Video** یا **DC**). Creator هدلس به‌صورت خودکار با آن هماهنگ می‌شود
4. لینک تماس را در فیلد ورودی پیست کنید
5. روی **GO** بزنید
6. منتظر وضعیت **Tunnel active** بمانید - از این به بعد تمام ترافیک دستگاه از طریق تماس عبور می‌کند

### تنظیمات

- **Tunnel mode** - VP8 (Video) یا DC
- **Name in call** - نامی که در تماس نمایش داده می‌شود (پیش‌فرض تصادفی)
- **Split tunneling** - انتخاب اپ‌هایی که از تونل عبور می‌کنند
- **Proxy settings** - پورت SOCKS5 و احراز هویت. حالت "Proxy only" فقط لیستنر SOCKS5 را بالا می‌آورد بدون فعال کردن VPN.
- **DNS settings** - DNS سیستم یا سفارشی
- **VP8 pacing** - بازنویسی پارامترهای pacing فریم VP8 (پایین را ببینید)
- **Reconnect on app start** - اتصال خودکار به آخرین لینک تماس هنگام اجرای اپ
- **Show logs** - نمایش پنل لاگ برای دیباگ

### تنظیم pacing VP8

تعیین می‌کند Joiner با چه نرخی فریم VP8 به SFU می‌فرستد. فقط روی Joiner تنظیم می‌شود؛ Creator مقادیر را در شروع نشست می‌گیرد.

- **Override VP8 pacing** - پیش‌فرض خاموش. با چک‌باکس خاموش، مقادیر پیش‌فرض `fps=24 batch=30` (سقف تئوریک ~6.5 Mbps). با روشن کردن، دو فیلد ظاهر می‌شود.
- **FPS** - نرخ اسمی فریم VP8. بازه 1..240. معمولاً 24-30.
- **Batch** - ضریب چگالی tick. نرخ ارسال واقعی ~ `fps x batch` فریم در ثانیه. بازه 1..256.

پهنای باند ~ `fps x batch x 1126 bytes/frame`. مثال‌ها:

| fps | batch | سقف |
|----:|------:|--------:|
| 24  | 1     | ~27 KB/s |
| 24  | 8     | ~216 KB/s |
| 24  | 30    | ~810 KB/s (~6.5 Mbps) |

batch بالاتر = بار CPU گوشی و SFU بیشتر. اگر در لاگ drop بسته می‌بینید یا اتصال ناپایدار است، batch را کم کنید.

### اگر کار نکرد

تغییر DNS را امتحان کنید: در تنظیمات اپ **DNS settings** را باز کنید و روی Custom قرار دهید (پیش‌فرض `8.8.8.8` / `8.8.4.4`). همچنین چک کنید DNS سیستم اندروید روی "Automatic" است (Private DNS خاموش).

## Joiner (آیفون)

روی iOS فقط حالت پراکسی SOCKS5 پشتیبانی می‌شود (VPN سیستمی ندارد). برای عبور تمام ترافیک دستگاه، از یک اپ VPN که SOCKS5 پشتیبانی می‌کند استفاده کنید (Happ، Shadowrocket، Streisand و غیره).

1. فایل `whitelist-bypass-proxy.ipa` (بدون امضا) را از [گیت‌هاب ریلیز](https://github.com/kulikov0/whitelist-bypass-iran/releases) دانلود و نصب کنید. برای نصب IPA بدون امضا روی iOS، AltStore، Sideloadly یا حساب دولوپر شخصی همگی کار می‌کنند. یا از سورس بیلد کنید (README را ببینید).
2. یک اپ VPN با پشتیبانی SOCKS5 نصب کنید (Happ روی App Store رایگان است).
3. اپ whitelist-bypass را باز کنید، حالت تونل را انتخاب کنید (**VP8** یا **DC**)، لینک تماس را پیست کنید و **Go** را بزنید. Creator هدلس با هر حالتی که انتخاب کنید هماهنگ می‌شود.
4. منتظر **Tunnel Active** بمانید. اپ یک endpoint محلی SOCKS5 نمایش می‌دهد (مثلاً `socks5://user:pass@127.0.0.1:1081`).
5. پارامترهای SOCKS5 را از whitelist-bypass کپی کنید.
6. در اپ VPN پیست کرده و وصل شوید - تمام ترافیک دستگاه از تونل عبور می‌کند.

### اتصال از طریق Happ

Happ یک اپ رایگان از App Store است که SOCKS5 را در قالب URL v2ray پشتیبانی می‌کند.

1. در whitelist-bypass منتظر **Tunnel Active** بمانید و روی **Copy v2ray URL** بزنید - یک لینک به‌شکل `socks://...@127.0.0.1:1081#WLB-1081` در کلیپ‌بورد قرار می‌گیرد.

   ![whitelist-bypass: Copy v2ray URL](../res/ios_interface.jpeg)

2. Happ را باز کنید، **+** بالا-راست را بزنید و **Import from clipboard** را انتخاب کنید.

   ![Happ: Import from clipboard](../res/ios_happ_s1.jpeg)

3. سرور `WLB-1081` در لیست ظاهر می‌شود - تونل را روشن کنید.

   ![Happ: تونل روشن](../res/ios_happ_s2.jpeg)

> اپ عمداً از NetworkExtension استفاده نمی‌کند، چون می‌خواهد با حساب دولوپر رایگان اپل نصب شود (بدون اشتراک سالانه 99 دلار). نقطه ضعف این روش: پورت SOCKS5 ممکن است بین اجراها تغییر کند - اگر پورت قبلی اشغال باشد، whitelist-bypass پورت آزاد انتخاب می‌کند. اگر بعد از ری‌استارت، تونل Happ کار نکرد، سرور `WLB-...` را پاک کنید و دوباره **Copy v2ray URL** + **Import from clipboard** بزنید.

به‌صورت جایگزین، می‌توانید پراکسی SOCKS5 را مستقیم در اپ‌های جداگانه تنظیم کنید:
- **Telegram**: Settings -> Data and Storage -> Proxy -> Add proxy -> SOCKS5
- یا در whitelist-bypass روی **Open in Telegram** بزنید تا خودکار تنظیم شود

### تنظیمات

- **Tunnel** -> **Mode** - VP8 یا DataChannel
- **Auth Mode** - Auto (یوزر/پسورد تصادفی) یا Manual (دستی)
- **Display Name** - نامی که در تماس دیده می‌شود
- **VP8 Pacing** - بازنویسی FPS و Batch (بخش pacing بالا را ببینید)
- **Show Logs** - پنل لاگ برای دیباگ

> محدودیت SOCKS5-only به‌خاطر قوانین اپل است: استفاده از NetworkExtension (VPN واقعی) به حساب دولوپر پولی نیاز دارد و با sideload رایگان کار نمی‌کند. اگر کسی از کامیونیتی نسخه‌ای با VPN کامل بر پایه این سورس بسازد - عالی است.

## Joiner (دسکتاپ: ویندوز / مک / لینوکس)

Joiner گرافیکی برای دسکتاپ. برخلاف joiner هدلس لینوکس پایین، نسخه دسکتاپ می‌تواند تونل VPN سیستمی بالا بیاورد: یک آداپتر TUN به‌علاوه split-default route که تمام ترافیک هاست از آن عبور می‌کند (مثل حالت VPN اندروید). ویندوز از wintun، لینوکس از `/dev/net/tun` + `iproute2` و مک از utun + `route(8)`/`ifconfig` استفاده می‌کند. اگر TUN لازم ندارید، یک چک‌باکس **SOCKS5 only** هست که فقط پراکسی محلی را بالا می‌آورد.

![رابط](../res/joiner_desktop_interface.jpeg)

### دانلود

از [گیت‌هاب ریلیز](https://github.com/kulikov0/whitelist-bypass-iran/releases):

- `WhitelistBypass Joiner-<version>-x64.exe` / `-ia32.exe` - ویندوز (پرتابل، بدون نصب‌کننده)
- `WhitelistBypass Joiner-<version>-x86_64.AppImage` - لینوکس
- `WhitelistBypass Joiner-<version>-arm64.dmg` / `-x64.dmg` - مک

### اجرا

1. لینک تماس را پیست کنید (`https://meet.bale.ai/i/<code>`)
2. به‌صورت اختیاری تغییر دهید: نام نمایشی، پورت SOCKS5، یوزر/پسورد SOCKS5، حالت تونل (**Video (VP8)** یا **DataChannel**)، VP8 FPS / batch، DNS آداپتر، حالت منابع
3. چک‌باکس **SOCKS5 only (no system-wide routing)** را تنظیم کنید:
   - خاموش (پیش‌فرض) - TUN بالا می‌آید، تمام ترافیک از تماس عبور می‌کند (نیاز به root/admin)
   - روشن - فقط SOCKS5 محلی، مثل iOS
4. روی **Start** کلیک کنید
5. منتظر `TUNNEL CONNECTED` در لاگ بمانید - بعد از آن SOCKS5 روی `127.0.0.1:<port>` فعال است و در حالت TUN تمام ترافیک هاست از تماس عبور می‌کند

### دسترسی‌ها

| سیستم عامل | چه چیزی لازم است | چطور بگیریم |
|---|---|---|
| Windows | Administrator | manifest فایل `.exe` به‌صورت خودکار UAC را می‌خواهد؛ تأیید کنید |
| Linux | root برای حالت TUN | `xhost +SI:localuser:root` اجرا کنید، سپس `sudo -E ./WhitelistBypass\ Joiner-*.AppImage --no-sandbox`. بدون root فقط حالت **SOCKS5 only** کار می‌کند |
| macOS | root برای حالت TUN | `sudo "/Applications/WhitelistBypass Joiner.app/Contents/MacOS/WhitelistBypass Joiner"`. بدون sudo فقط حالت **SOCKS5 only** کار می‌کند |

### چه زمانی دسکتاپ به‌جای اندروید/آیفون

- VPN سیستمی روی لپ‌تاپ یا PC لازم دارید
- رابط گرافیکی می‌خواهید و CLI هدلس پایین برای شما مناسب نیست
- روی یک سیستم دسکتاپ هستید و دستگاه اندروید/iOS سالم در دسترس نیست

## Joiner (لینوکس، هدلس)

Joiner هدلس برای سرورها و دسکتاپ‌های لینوکس. یک پراکسی SOCKS5 محلی بالا می‌آورد. هر کلاینت SOCKS5 را به آن وصل کنید (`curl --socks5`، تلگرام، یا کل سیستم از طریق `redsocks` / `tun2socks`).

از سورس بیلد کنید (فعلاً باینری پیش‌ساخته منتشر نمی‌شود - `./build-headless.sh` در README).

### اجرا

```sh
./headless-bale-joiner --join-link https://meet.bale.ai/i/<code> --socks-port 1080
```

بعد از `TUNNEL CONNECTED`، لیستنر SOCKS5 روی `127.0.0.1:<socks-port>` فعال است. تست سریع:

```sh
curl --socks5 127.0.0.1:1080 https://api.ipify.org
```

### پرچم‌ها

| پرچم | توضیحات |
|---|---|
| `--join-link <link>` | `https://meet.bale.ai/i/<code>` (الزامی) |
| `--name <str>` | نام نمایشی در تماس (پیش‌فرض `Joiner`) |
| `--socks-port <port>` | پورت SOCKS5 (پیش‌فرض `1080`) |
| `--socks-user <user>` | یوزرنیم SOCKS5 (اختیاری) |
| `--socks-pass <pass>` | پسورد SOCKS5 (اختیاری) |
| `--resources <mode>` | `default` / `moderate` / `unlimited` |
| `--tunnel-mode <mode>` | `vp8` یا `dc` (پیش‌فرض `vp8`) |
| `--vp8-fps <fps>` | نرخ فریم VP8 (پیش‌فرض `24`) |
| `--vp8-batch <n>` | ضریب batch فریم (پیش‌فرض `30`) |

با تنظیم `--socks-user`/`--socks-pass`، پراکسی SOCKS5 نیاز به احراز هویت دارد. بدون آن‌ها روی `127.0.0.1` باز است.

### تونل سیستمی (به‌سبک VPN اندروید)

برای عبور دادن تمام ترافیک هاست از پراکسی، `tun2socks` یا `redsocks` را روی SOCKS5 محلی اجرا کنید. مثال با `tun2socks`:

```sh
sudo tun2socks -device tun://bale0 -proxy socks5://127.0.0.1:1080
```
