<p align="center">
  <img src="build/appicon.png" width="120" alt="PAMPP logo">
</p>

<h1 align="center">PAMPP</h1>

<p align="center">
  Windows için hafif, hızlı ve portable PHP geliştirme ortamı.<br>
  Nginx / Apache · PHP 7.4 – 8.5 · MariaDB · phpMyAdmin · Composer · Yerel SSL
</p>

<p align="center">
  <a href="https://github.com/ercanulucandev/pampp/releases/latest"><b>⬇️ İndir (pampp.exe)</b></a> ·
  <a href="#geliştirme">Kaynaktan derle</a> ·
  <a href="LICENSE">MIT Lisansı</a>
</p>

---

## Neden PAMPP?

XAMPP, Laragon, MAMP gibi araçlar ya ağır ya da esnek değil. PAMPP **tek bir ~13 MB'lık exe**. Kurulum gerektirmez, sisteme hiçbir şey yazmaz; indirdiği her şeyi kendi klasöründe tutar.

## Özellikler

- **Otomatik siteler:** `www/blog` klasörü anında `blog.test` olur. `public/` klasörü varsa (Laravel, Symfony…) kök dizin o olur.
- **Site başına PHP sürümü:** Tek tıkla değişir, sunucu kesintisiz yeniden yüklenir.
- **Birden fazla PHP:** 7.4'ten 8.5'e kadar. Her sürüm kendi FastCGI havuzunda çalışır; sadece kullanılanlar başlatılır.
- **Yerel SSL:** PAMPP kendi sertifika otoritesini oluşturur. Tüm siteler `https://` ile kilit simgesiyle açılır. Sertifikalar site eklendikçe otomatik yenilenir; güven onayı tek tık, yönetici yetkisi gerekmez.
- **Nginx ↔ Apache:** Ayarlardan geçiş; ikisi de aynı siteleri, PHP havuzlarını ve sertifikayı kullanır.
- **Tek tıkla güncelleme:** PHP, Nginx, Apache, MariaDB, phpMyAdmin ve Composer resmi kaynaklardan kurulur ve güncellenir; ayarlar korunur.
- **PHP eklentileri:** Arayüzden aç/kapat, `php.ini`'ye tek tıkla erişim.
- **Başlangıç sayfası:** `https://localhost` adresinde siteler, sürümler, veritabanı bilgileri ve ilk adımlar.
- **Şık yeni site akışı:** Yeni site oluşturunca hazır bir karşılama sayfası gelir (PHP sürümü, DB bağlantısı, HTTPS durumu, sonraki adımlar).
- **hosts dosyası:** Otomatik güncellenir (yalnızca o adımda UAC onayı istenir). `.localhost` uzantısı seçilirse hosts dosyasına hiç dokunulmaz.
- **Sistem tepsisi:** Kapatınca tepsiye küçülür, servisler çalışmaya devam eder.
- **Güvenli süreç yönetimi:** Tüm alt süreçler bir Windows Job Object içinde çalışır; PAMPP kapanırsa, çökse bile, arkada nginx/php kalmaz.
- **Terminal:** `php`, `composer` ve `mariadb` PATH'te hazır gelir.

## Hızlı başlangıç

1. [Releases](https://github.com/ercanulucandev/pampp/releases/latest) sayfasından `pampp.exe`'yi indir.
2. Bir klasöre koy, mesela `C:\pampp\`. OneDrive veya Masaüstü önerilmez, çünkü indirilen bileşenler senkronlanmaya çalışır.
3. Çalıştır ve **Hızlı kurulumu başlat**'a tıkla. Nginx, iki PHP sürümü, MariaDB, phpMyAdmin ve Composer kurulur, servisler başlar, başlangıç sayfası açılır.
4. Ayarlar → SSL → **Güvenilir yap** ile HTTPS uyarılarından kurtul.

> PHP ve Apache için [Visual C++ Redistributable](https://aka.ms/vs/17/release/vc_redist.x64.exe) gerekir. PAMPP eksikse uyarır.

## Klasör yapısı (exe'nin yanında)

```
pampp.exe
pampp.json      ayarlar
bin/            php/8.3, php/8.4, nginx, apache, mariadb, phpmyadmin, composer
etc/            üretilen nginx/apache/mariadb config'leri (her seferinde yeniden yazılır)
www/            projeler
data/mariadb    veritabanı dosyaları
data/ssl        yerel sertifika otoritesi ve site sertifikası
data/start      başlangıç sayfası (localhost)
logs/           tüm loglar
```

Portlar: HTTP 80, HTTPS 443, MariaDB 3306. PHP işçileri `10000 + (major*10+minor)*10 + n` portlarını kullanır (ör. 8.3 → 10830–10833).

## Geliştirme

Gerekenler: Go 1.25+, Node 20+, Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

```bash
wails dev      # canlı geliştirme
wails build    # build/bin/pampp.exe
```

Uçtan uca test (gerçek bileşenleri indirir, yaklaşık 250 MB). Nginx ve Apache üzerinde PHP sürüm geçişini, MariaDB'yi, phpMyAdmin'i, HTTPS'i ve yeni site akışını doğrular:

```bash
PAMPP_E2E_ROOT=/tmp/pampp-e2e go test ./internal/core -run TestEndToEnd -v
```

### Kod yapısı

- `internal/core/manager.go`: Durum, servis orkestrasyonu, kurulumlar
- `internal/core/php.go`: php-cgi havuzu, php.ini ve eklentiler
- `internal/core/webserver.go`: Nginx/Apache config üretimi ve süreç kontrolü
- `internal/core/ssl.go`: Yerel CA, site sertifikaları, Windows güven deposu
- `internal/core/pages.go`, `templates/`: Başlangıç sayfası ve yeni site karşılama sayfası
- `internal/core/mariadb.go`, `hosts.go`, `catalog.go` (sürüm kaynakları), `download.go`
- `app.go`: Frontend'e açılan metotlar · `tray.go`: Sistem tepsisi
- `frontend/src/views`: Panel, Siteler, PHP, Bileşenler, Loglar, Ayarlar

## Lisans

[MIT](LICENSE)
