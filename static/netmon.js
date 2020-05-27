const intv = 3;
let host = '/';
const F = v => {
    v = v || 0;
    let k = 'K';
    if (v / 1024 > 1) { v /= 1024; k = 'M'; }
    if (v / 1024 > 1) { v /= 1024; k = 'G'; }
    if (v / 1024 > 1) { v /= 1024; k = 'T'; }
    if (v / 1024 > 1) { v /= 1024; k = 'P'; }
    return `${v.toFixed(2)} ${k}`;
}
window.addEventListener("load", () => {
    let byname = '';
    let refresh = false;
    const $ = (...args) => document.querySelector(...args);
    const table = $("#container");
    table.innerHTML = `
<div class="custom-control custom-switch" style="float:right;clear:both;">
  <input type="checkbox" class="custom-control-input" id="refresh0">
  <label class="custom-control-label" for="refresh0" id="refresh1">Auto Refresh (OFF)</label>
</div>
<br />
<table class="table">
    <thead><tr>
        <th scope="col">VM ID<button onclick="by('')" style="margin-left:12px;" type="button" class="btn btn-light btn-sm">↑</button></th>
        <th scope="col">Rx<button onclick="by('rx_rate')" style="margin-left:12px;" type="button" class="btn btn-light btn-sm">↓</button></th>
        <th scope="col">Tx<button onclick="by('tx_rate')" style="margin-left:12px;" type="button" class="btn btn-light btn-sm">↓</button></th>
        <th scope="col">Total Rx<button onclick="by('rx')" style="margin-left:12px;" type="button" class="btn btn-light btn-sm">↓</button></th>
        <th scope="col">Total Tx<button onclick="by('tx')" style="margin-left:12px;" type="button" class="btn btn-light btn-sm">↓</button></th>
        <th scope="col">Actions</th>
    </tr></thead>
    <tbody id="updater"></tbody>
</table>`;

    window.by = (name) => {
        byname = name;
        // const switch0 = $("#refresh0");
        // switch0.checked = false;
        // switch0.onchange();
        reload();
    }
    
    window.ban = async (name, banned) => {
        console.log(name, banned);
        const data = await (await fetch(`${host}ban`, {
            method: 'POST',
            credentials: 'same-origin',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({name, banned})
        })).json();
        if (data.banned !== undefined) reload();
    }

    const reload = async () => {
        const updater = $("#updater");
        const data = await (await fetch(`${host}stats`)).json();
        const ifce = data.interfaces || {};
        const iv = data.interval || intv;
        let html = '';
        let ranks = Object.keys(ifce);
        if (byname !== '') {
            ranks = ranks.sort((a,b) => (ifce[b][byname] || 0) - (ifce[a][byname] || 0) > 0 ? 1 : -1);
        }
        ranks = ranks.sort((a,b) => (ifce[a].banned && ifce[b].banned) ? 0 : (ifce[a].banned ? -1 : (ifce[b].banned ? 1 : 0)));
        ranks = ranks.sort((a,b) => (ifce[a].white && ifce[b].white) ? 0 : (ifce[a].white ? -1 : (ifce[b].white ? 1 : 0)));
        let trx = 0, ttx = 0;
        ranks.forEach(k => {
            const i = ifce[k];
            trx += i.rx_rate || 0;
            ttx += i.tx_rate || 0;
            let addons = "";
            if (i.banned) addons = 'style="color: red;"';
            if (i.white) addons = 'style="color: green;"';
            txt = `<th scope="row" ${addons}>${i.name}</th><td>${F(i.rx_rate)}bps</td><td>${F(i.tx_rate)}bps</td><td>${F(i.rx/8)}B</td><td>${F(i.tx/8)}B</td>`;
            act = `<td>
                <button onclick="ban('${i.name}', 1)" type="button" class="btn btn-sm btn-danger">Limit</button>
                <button onclick="ban('${i.name}', -20)" type="button" class="btn btn-sm btn-success">White</button>
                <button onclick="ban('${i.name}', -1)" type="button" class="btn btn-sm btn-warning">Reset</button>
            </td>`;
            html += `<tr>${txt}${act}</tr>`;
        });
        html = `<tr><th scope="row">TOTAL</th><td>${F(trx)}bps</td><td>${F(ttx)}bps</td><td>-</td><td>-</td></tr>` + html;
        updater.innerHTML = html;
    };
    
    setTimeout(() => {
        const switch0 = $("#refresh0");
        const switch1 = $("#refresh1");
        switch0.onchange = () => {
            if (switch0.checked) switch1.innerText = `Auto Refresh (ON)`;
            else switch1.innerText = `Auto Refresh (OFF)`;
            refresh = switch0.checked;
        };
        
        setInterval(async () => {
            if (!refresh) return;
            await reload();
        }, intv * 1000);
        reload();
    }, 100);
    
});
