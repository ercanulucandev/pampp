<?php
// {{SITE}} — PAMPP tarafından oluşturuldu. Bu dosyayı silip kendi projenle değiştirebilirsin.
$site = '{{SITE}}';
$host = preg_replace('/:\d+$/', '', $_SERVER['HTTP_HOST'] ?? $site);
$db = null;
if (function_exists('mysqli_connect')) {
    mysqli_report(MYSQLI_REPORT_OFF);
    $conn = @mysqli_connect('127.0.0.1', 'root', '', '', {{DBPORT}});
    if ($conn) { $db = mysqli_get_server_info($conn); mysqli_close($conn); }
}
$https = !empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off';
$exts = get_loaded_extensions();
sort($exts);
?>
<!doctype html>
<html lang="tr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title><?= htmlspecialchars($host) ?> · hazır</title>
<style>
:root{--bg:#f6f7fb;--panel:#fff;--border:#e6e8ef;--text:#0f172a;--muted:#64748b;--accent:#6d5df6;--soft:#efedff;--green:#16a34a;--gsoft:#e8f7ee;--red:#dc2626;--rsoft:#fdecec}
@media (prefers-color-scheme:dark){:root{--bg:#0d0f14;--panel:#151821;--border:#252a36;--text:#e7e9ee;--muted:#8b93a7;--accent:#9d8cff;--soft:#211f3d;--green:#22c55e;--gsoft:#10291b;--red:#f87171;--rsoft:#33191b}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;background:var(--bg);color:var(--text);font:15px/1.55 "Segoe UI Variable Text","Segoe UI",system-ui,sans-serif;display:grid;place-items:center;padding:32px 20px}
.box{width:100%;max-width:760px}
.top{display:flex;align-items:center;gap:16px;margin-bottom:22px}
.mark{width:58px;height:58px;border-radius:16px;display:grid;place-items:center;font-size:28px;background:linear-gradient(135deg,#6366f1,#8b5cf6 55%,#c026d3);box-shadow:0 12px 30px rgba(109,93,246,.35)}
h1{margin:0;font-size:30px;letter-spacing:-.02em}.top p{margin:2px 0 0;color:var(--muted)}
.card{background:var(--panel);border:1px solid var(--border);border-radius:16px;padding:22px;margin-bottom:16px}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
@media (max-width:640px){.stats{grid-template-columns:repeat(2,1fr)}}
.stat{border:1px solid var(--border);border-radius:12px;padding:12px 14px}.stat small{color:var(--muted);display:block;font-size:12.5px}.stat b{font-size:16px}
.ok{color:var(--green)}.bad{color:var(--red)}
h2{font-size:15px;margin:0 0 12px}
code{font:13px "Cascadia Code",Consolas,monospace;background:var(--soft);color:var(--accent);padding:2px 6px;border-radius:6px;word-break:break-all}
ol{margin:0;padding-left:20px}li{margin:8px 0}
.exts{display:flex;flex-wrap:wrap;gap:6px}.exts span{font-size:12px;padding:3px 9px;border-radius:999px;background:var(--soft);color:var(--accent)}
details summary{cursor:pointer;color:var(--muted);font-size:14px}
footer{text-align:center;color:var(--muted);font-size:13px;margin-top:6px}
</style>
</head>
<body>
<div class="box">
  <div class="top">
    <div class="mark">🎉</div>
    <div>
      <h1><?= htmlspecialchars($host) ?> hazır!</h1>
      <p>Siten PAMPP üzerinde çalışıyor. Bu sayfayı kendi kodunla değiştirerek başlayabilirsin.</p>
    </div>
  </div>

  <div class="card">
    <div class="stats">
      <div class="stat"><small>PHP</small><b><?= PHP_VERSION ?></b></div>
      <div class="stat"><small>Sunucu</small><b><?= htmlspecialchars(explode(' ', $_SERVER['SERVER_SOFTWARE'] ?? 'PAMPP')[0]) ?></b></div>
      <div class="stat"><small>Veritabanı</small><b class="<?= $db ? 'ok' : 'bad' ?>"><?= $db ? '● Bağlı' : '● Kapalı' ?></b></div>
      <div class="stat"><small>Bağlantı</small><b class="<?= $https ? 'ok' : '' ?>"><?= $https ? '🔒 HTTPS' : 'HTTP' ?></b></div>
    </div>
  </div>

  <div class="card">
    <h2>✨ Sırada ne var?</h2>
    <ol>
      <li>Bu dosyayı düzenle: <code><?= htmlspecialchars(__FILE__) ?></code></li>
      <li>Laravel mi kuracaksın? Klasörü boşalt, PAMPP'ta bu site için Terminal'i aç ve şunu çalıştır: <code>composer create-project laravel/laravel .</code> (<code>public</code> klasörü otomatik kök dizin olur.)</li>
      <li>Veritabanı: <code>127.0.0.1:{{DBPORT}}</code>, kullanıcı <code>root</code>, şifre boş<?= $db ? ' (MariaDB ' . htmlspecialchars($db) . ')' : '' ?>.</li>
      <li>PHP sürümünü PAMPP'taki <b>Siteler</b> sayfasından tek tıkla değiştirebilirsin.</li>
    </ol>
  </div>

  <div class="card">
    <details>
      <summary>Yüklü PHP eklentileri (<?= count($exts) ?>)</summary>
      <div class="exts" style="margin-top:12px"><?php foreach ($exts as $e): ?><span><?= htmlspecialchars($e) ?></span><?php endforeach ?></div>
    </details>
  </div>
  <footer>PAMPP ile oluşturuldu ⚡</footer>
</div>
</body>
</html>
