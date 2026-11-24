(function () {
    var form = document.getElementById('loginForm');
    if (!form) return;
    var errEl = document.getElementById('loginErr');
    var btn = document.getElementById('loginBtn');

    form.addEventListener('submit', function (e) {
        e.preventDefault();
        var user = document.getElementById('user').value;
        var pass = document.getElementById('pass').value;

        btn.disabled = true;
        errEl.classList.add('d-none');

        fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ user: user, pass: pass })
        })
            .then(function (r) {
                return r.json().then(function (d) { return { status: r.status, d: d }; });
            })
            .then(function (res) {
                if (res.status === 200 && res.d && res.d.success) {
                    window.location.href = '/';
                } else {
                    errEl.textContent = (res.d && res.d.message) || '登录失败';
                    errEl.classList.remove('d-none');
                    btn.disabled = false;
                }
            })
            .catch(function () {
                errEl.textContent = '请求失败，请检查网络';
                errEl.classList.remove('d-none');
                btn.disabled = false;
            });
    });
})();
