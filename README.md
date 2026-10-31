<p align="center">
  <img src="build/appicon.png" width="120" alt="PAMPP logo">
</p>

<h1 align="center">PAMPP</h1>

<p align="center">
  Windows için hafif ve hızlı PHP + Node.js geliştirme ortamı. Portable ya da kurulumlu.<br>
  Nginx / Apache · PHP 7.4 – 8.5 · Node.js · MariaDB · phpMyAdmin · Composer · Yerel SSL
</p>

<p align="center">
  <a href="https://github.com/ercanulucandev/pampp/releases/latest/download/pampp-setup.exe"><b>⬇️ Installer (pampp-setup.exe)</b></a> ·
  <a href="https://github.com/ercanulucandev/pampp/releases/latest/download/pampp.exe"><b>Portable (pampp.exe)</b></a> ·
  <a href="#geliştirme">Kaynaktan derle</a> ·
  <a href="LICENSE">MIT Lisansı</a>
</p>

---

## Neden PAMPP?

XAMPP, Laragon, MAMP gibi araçlar ya ağır ya da esnek değil. PAMPP **~14 MB'lık tek bir program**. İster kur, ister portable çalıştır; indirdiği her şeyi kendi klasöründe tutar.

## Özellikler

### PHP
- **Otomatik siteler:** `www/blog` klasörü anında `blog.test` olur. `public/` klasörü varsa (Laravel, Symfony…) kök dizin o olur.
- **Site başına PHP sürümü:** Tek tıkla değişir, sunucu kesintisiz yeniden yüklenir.
- **Birden fazla PHP:** 7.4'ten 8.5'e kadar. Her sürüm kendi FastCGI havuzunda çalışır; sadece kullanılanlar başlatılır.
- **Nginx ↔ Apache:** Ayarlardan geçiş; ikisi de aynı siteleri, PHP havuzlarını ve sertifikayı kullanır.
- **PHP eklentileri:** Arayüzden aç/kapat, `php.ini`'ye tek tıkla erişim.
- **Şık yeni site akışı:** Yeni siteyle birlikte hazır bir karşılama sayfası gelir: PHP sürümü, DB bağlantısı, HTTPS durumu, sonraki adımlar.

### Node.js
- **Otomatik proje algılama:** `nodejs/` içindeki her `package.json` projesi listelenir. Framework (Next.js, Vite, Nuxt, Astro, SvelteKit, Express, NestJS…) ve paket yöneticisi (npm, pnpm, yarn) tanınır.
- **Tek tıkla script'ler:** Paketleri kur; `dev`, `build`, `start`, `lint` dahil tüm script'leri başlat ve durdur. Çıktıyı canlı izle. Dev sunucusunun adresi otomatik bulunur ve tek tıkla açılır.
- **Node sürümleri:** LTS ve Current sürümleri kur, proje başına Node sürümü seç.
- **Proje şablonları:** React / Vue / Svelte + Vite, Next.js, Express API veya boş proje. Oluşturulur ve paketleri otomatik kurulur.

### Veritabanı
- **Bağlantı bilgileri:** Tek tıkla kopyalanır; Laravel için hazır `.env` örneği de var.
- **Root şifresi:** Değiştirilir; PAMPP ve phpMyAdmin otomatik güncellenir.
- **Veritabanları:** Boyut ve tablo sayısıyla listelenir; oluşturulur, silinir. Tablolar ve tablo verisi görüntülenir; tablolar boşaltılır veya silinir.
- **Kullanıcılar:** Oluştur (güçlü şifre üretici ile), sadece belirli bir veritabanına yetki ver, şifre değiştir, sil.
- **Yedek ve içe aktarma:** Tek tıkla `.sql` yedek alınır (`data/backups`), `.sql` dosyası istenen veritabanına yüklenir.
- **SQL konsolu:** Sorgu çalıştır, sonuçları tablo olarak gör (Ctrl + Enter).

### Genel
- **Yerel SSL:** PAMPP kendi sertifika otoritesini oluşturur. Tüm siteler `https://` ile kilit simgesiyle açılır. Sertifikalar otomatik yenilenir. Kurulumdan sonra PAMPP sertifikaya güvenmeyi adım adım anlatan bir pencere açar; tek tık yeter, yönetici yetkisi gerekmez.
- **Tek tıkla güncelleme:** PHP, Node.js, Nginx, Apache, MariaDB, phpMyAdmin ve Composer resmi kaynaklardan kurulur ve güncellenir; ayarlar korunur.
- **Başlangıç sayfası:** `https://localhost` adresinde siteler, sürümler, veritabanı bilgileri ve ilk adımlar.
- **hosts dosyası:** Otomatik güncellenir (yalnızca o adımda UAC onayı istenir). `.localhost` uzantısı seçilirse hosts dosyasına hiç dokunulmaz.
- **Sistem tepsisi, canlı loglar, hazır terminal:** Terminalde `php`, `node`, `composer` ve `mariadb` PATH'te gelir.
- **Güvenli süreç yönetimi:** Tüm alt süreçler bir Windows Job Object içinde çalışır; PAMPP kapanırsa, çökse bile, arkada süreç kalmaz.

## Hızlı başlangıç

İki seçenek var, ikisi de aynı programdır:

| | Installer | Portable |
|---|---|---|
| Dosya | `pampp-setup.exe` | `pampp.exe` |
| Kurulum | `C:\pampp` (değiştirilebilir), başlat menüsü ve masaüstü kısayolu | Yok, istediğin klasörden çalıştır |
| Veriler (www, nodejs, veritabanı) | `C:\pampp` içinde; kaldırınca silinmez | exe'nin yanındaki klasörler |

1. [Releases](https://github.com/ercanulucandev/pampp/releases/latest) sayfasından birini indir. Portable sürümü OneDrive veya Masaüstü yerine kendi klasörüne koy (ör. `D:\pampp\`).
2. Çalıştır ve **Hızlı kurulumu başlat**'a tıkla. Nginx, iki PHP sürümü, MariaDB, phpMyAdmin, Composer ve Node.js LTS kurulur; servisler başlar ve başlangıç sayfası açılır.
3. Ayarlar → SSL → **Güvenilir yap** ile HTTPS uyarılarından kurtul.

> PHP ve Apache için [Visual C++ Redistributable](https://aka.ms/vs/17/release/vc_redist.x64.exe) gerekir. PAMPP eksikse uyarır.
> Program imzasız olduğu için Windows SmartScreen ilk açılışta uyarabilir: *Ek bilgi → Yine de çalıştır*.

## Klasör yapısı

Installer sürümünde `C:\pampp`, portable sürümde exe'nin yanındaki klasör. (Program Files altına kurulursa veriler `%USERPROFILE%\PAMPP`'a yazılır.)

```
pampp.json      ayarlar
bin/            php/8.4, node/26, nginx, apache, mariadb, phpmyadmin, composer
etc/            üretilen nginx/apache/mariadb config'leri (her seferinde yeniden yazılır)
www/            PHP projeleri
nodejs/         Node.js projeleri
data/mariadb    veritabanı dosyaları
data/ssl        yerel sertifika otoritesi ve site sertifikası
data/start      başlangıç sayfası (localhost)
logs/           tüm loglar
```

Portlar: HTTP 80, HTTPS 443, MariaDB 3306. PHP işçileri `10000 + (major*10+minor)*10 + n` portlarını kullanır (ör. 8.3 → 10830–10833).

## Geliştirme

Gerekenler: Go 1.25+, Node 20+, Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`). Installer için ayrıca NSIS (`winget install NSIS.NSIS`).

```bash
wails dev           # canlı geliştirme
wails build         # build/bin/pampp.exe
wails build -nsis   # + build/bin/PAMPP-setup.exe
```

Uçtan uca testler gerçek bileşenleri indirir. Nginx ve Apache üzerinde PHP sürüm geçişini, MariaDB'yi, phpMyAdmin'i, HTTPS'i, yeni site akışını ve Node.js projelerini (Express dev sunucusu, Vite build, istersen Next.js) doğrular:

```bash
PAMPP_E2E_ROOT=/tmp/pampp-e2e go test ./internal/core -v
PAMPP_E2E_ROOT=/tmp/pampp-e2e PAMPP_E2E_NEXT=1 go test ./internal/core -run TestNodeEndToEnd -v
```

### Kod yapısı

- `internal/core/manager.go`: Durum, servis orkestrasyonu, kurulumlar
- `internal/core/php.go`: php-cgi havuzu, php.ini ve eklentiler
- `internal/core/db.go`: Veritabanı yönetimi (veritabanları, tablolar, kullanıcılar, yedek, SQL)
- `internal/core/node.go`: Node.js sürümleri, proje algılama, script çalıştırıcı, şablonlar
- `internal/core/webserver.go`: Nginx/Apache config üretimi ve süreç kontrolü
- `internal/core/ssl.go`: Yerel CA, site sertifikaları, Windows güven deposu
- `internal/core/pages.go`, `templates/`: Başlangıç sayfası ve yeni site karşılama sayfası
- `internal/core/paths.go`: Portable / kurulu mod ve veri klasörü
- `internal/core/mariadb.go`, `hosts.go`, `catalog.go` (sürüm kaynakları), `download.go`
- `app.go`: Frontend'e açılan metotlar · `tray.go`: Sistem tepsisi
- `frontend/src/views`: Panel, Siteler, PHP, Node.js, Veritabanı, Bileşenler, Loglar, Ayarlar
- `build/windows/installer/project.nsi`: NSIS installer

## Lisans

[MIT](LICENSE)
