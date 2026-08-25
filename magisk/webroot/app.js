const outputView = document.getElementById('output-view');

function showOutput(msg) {
  outputView.textContent = msg;
}

async function runCmd(cmd) {
  try {
    if (window.ksu && typeof window.ksu.exec === 'function') {
      const raw = await window.ksu.exec(cmd);
      if (typeof raw === 'object' && raw !== null) {
        return {
          stdout: raw.stdout !== undefined ? String(raw.stdout) : '',
          stderr: raw.stderr !== undefined ? String(raw.stderr) : '',
          code: raw.errno !== undefined ? raw.errno : (raw.code !== undefined ? raw.code : 0)
        };
      }
      if (typeof raw === 'string') {
        try {
          const parsed = JSON.parse(raw);
          if (typeof parsed === 'object' && parsed !== null) {
            return {
              stdout: parsed.stdout !== undefined ? String(parsed.stdout) : '',
              stderr: parsed.stderr !== undefined ? String(parsed.stderr) : '',
              code: parsed.errno !== undefined ? parsed.errno : (parsed.code !== undefined ? parsed.code : 0)
            };
          }
        } catch (e) {
          return { stdout: raw, stderr: '', code: 0 };
        }
      }
      return { stdout: String(raw || ''), stderr: '', code: 0 };
    }
    const resp = await fetch('/api/exec', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cmd })
    });
    return resp.ok ? await resp.json() : { stdout: '', stderr: 'HTTP ' + resp.status, code: 1 };
  } catch (err) {
    return { stdout: '', stderr: String(err), code: 1 };
  }
}

document.getElementById('btn-start').onclick = async () => {
  showOutput('デーモンを起動中...');
  const res = await runCmd('nohup /system/bin/netclient daemon >> /data/adb/netclient/netclient.log 2>&1 &');
  showOutput((res.stdout || '') + '\n' + (res.stderr || '') + '\n[完了] 起動コマンドを実行しました');
};

document.getElementById('btn-restart').onclick = async () => {
  showOutput('デーモンを再起動中...');
  const res = await runCmd('[ -f /data/adb/netclient/netclient.pid ] && kill -HUP $(cat /data/adb/netclient/netclient.pid) 2>/dev/null || nohup /system/bin/netclient daemon >> /data/adb/netclient/netclient.log 2>&1 &');
  showOutput((res.stdout || '') + '\n' + (res.stderr || '') + '\n[完了] 再起動コマンドを実行しました');
};

document.getElementById('btn-stop').onclick = async () => {
  showOutput('デーモンを停止中...');
  const res = await runCmd('[ -f /data/adb/netclient/netclient.pid ] && kill $(cat /data/adb/netclient/netclient.pid) 2>/dev/null || true');
  showOutput((res.stdout || '') + '\n' + (res.stderr || '') + '\n[完了] 停止コマンドを実行しました');
};

document.getElementById('btn-reapply').onclick = async () => {
  showOutput('DNS/PBRルールを再適用中...');
  const res = await runCmd(`
    iptables -t nat -D OUTPUT -p udp --dport 53 ! -d 127.0.0.1 -m owner ! --uid-owner 0 -j DNAT --to-destination 127.0.0.1:5300 2>/dev/null || true
    iptables -t nat -A OUTPUT -p udp --dport 53 ! -d 127.0.0.1 -m owner ! --uid-owner 0 -j DNAT --to-destination 127.0.0.1:5300
    ip6tables -t nat -D OUTPUT -p udp --dport 53 ! -d ::1 -m owner ! --uid-owner 0 -j DNAT --to-destination [::1]:5300 2>/dev/null || true
    ip6tables -t nat -A OUTPUT -p udp --dport 53 ! -d ::1 -m owner ! --uid-owner 0 -j DNAT --to-destination [::1]:5300 2>/dev/null || true
  `);
  showOutput((res.stdout || '') + '\n' + (res.stderr || '') + '\n[完了] DNS/PBRルールを再適用しました');
};

document.getElementById('btn-register').onclick = async () => {
  const token = document.getElementById('token-input').value.trim();
  if (!token) return alert('Tokenを入力してください');
  showOutput('ネットワーク登録中 (netclient register -t)...');
  const res = await runCmd(`netclient register -t "${token}"`);
  const output = (res.stdout || '') + (res.stderr ? '\n[stderr] ' + res.stderr : '');
  showOutput(output || (res.code === 0 ? '[成功] ネットワーク登録が完了しました' : '[エラー] 登録に失敗しました (code ' + res.code + ')'));
  if (res.code === 0) {
    document.getElementById('token-input').value = '';
  }
};

document.getElementById('btn-leave').onclick = async () => {
  const net = document.getElementById('leave-input').value.trim();
  if (!net) return alert('ネットワーク名を入力してください');
  if (!confirm(`ネットワーク "${net}" から離脱しますか？`)) return;
  showOutput(`ネットワーク "${net}" から離脱中...`);
  const res = await runCmd(`netclient leave -n "${net}"`);
  const output = (res.stdout || '') + (res.stderr ? '\n[stderr] ' + res.stderr : '');
  showOutput(output || (res.code === 0 ? '[成功] 離脱が完了しました' : '[エラー] 離脱に失敗しました (code ' + res.code + ')'));
  if (res.code === 0) {
    document.getElementById('leave-input').value = '';
  }
};

async function loadBypassConfig() {
  const res = await runCmd('cat /data/adb/netclient/bypass.json 2>/dev/null || true');
  if (res.stdout && res.stdout.trim()) {
    try {
      const cfg = JSON.parse(res.stdout.trim());
      if (Array.isArray(cfg.home_ssids)) {
        document.getElementById('bypass-ssids').value = cfg.home_ssids.join(', ');
      }
      if (Array.isArray(cfg.bypass_subnets)) {
        document.getElementById('bypass-subnets').value = cfg.bypass_subnets.join(', ');
      }
    } catch (e) {}
  }
}

document.getElementById('btn-save-bypass').onclick = async () => {
  const rawSsids = document.getElementById('bypass-ssids').value;
  const rawSubnets = document.getElementById('bypass-subnets').value;
  const home_ssids = rawSsids.split(',').map(s => s.trim()).filter(Boolean);
  const bypass_subnets = rawSubnets.split(',').map(s => s.trim()).filter(Boolean);
  const jsonStr = JSON.stringify({ home_ssids, bypass_subnets }, null, 2);
  showOutput('バイパス設定を保存中...');
  const res = await runCmd(`
    cat << 'EOF' > /data/adb/netclient/bypass.json
${jsonStr}
EOF
    chmod 0644 /data/adb/netclient/bypass.json
    [ -f /data/adb/netclient/netclient.pid ] && kill -HUP $(cat /data/adb/netclient/netclient.pid) 2>/dev/null || true
  `);
  showOutput((res.stdout || '') + '\n' + (res.stderr || '') + '\n[完了] バイパス設定を保存し適用しました');
};

loadBypassConfig();

