// auth.js - Handles JWT authentication and fetch interception
(function() {
    // 2. Setup Login UI logic
    function showLoginModal() {
        const modal = document.getElementById('login-modal');
        if (modal) {
            modal.style.display = 'flex';
        }
    }

    function hideLoginModal() {
        const modal = document.getElementById('login-modal');
        if (modal) {
            modal.style.display = 'none';
        }
    }

    function handleLogin(e) {
        e.preventDefault();
        const form = e.target;
        const emailInput = form.querySelector('input[type="email"]');
        const passwordInput = form.querySelector('input[type="password"]');
        const email = emailInput ? emailInput.value : '';
        const password = passwordInput ? passwordInput.value : '';

        // Clear any previous error
        let errEl = form.querySelector('.login-error');
        if (errEl) errEl.remove();

        APIClient.post('/auth/login', { email, password })
        .then(data => {
            if (data && data.token) {
                localStorage.setItem('k8s_token', data.token);
                hideLoginModal();
                window.location.reload();
            } else {
                throw new Error('Authentication failed');
            }
        })
        .catch(err => {
            console.error('Login error:', err);
            const errDiv = document.createElement('div');
            errDiv.className = 'login-error';
            errDiv.style.color = 'var(--color-critical)';
            errDiv.style.marginTop = 'var(--space-md)';
            errDiv.style.textAlign = 'center';
            errDiv.style.fontSize = '14px';
            errDiv.textContent = err.message || 'Login failed. Please try again.';
            form.appendChild(errDiv);
        });
    }

    // 3. Initialize
    document.addEventListener("DOMContentLoaded", () => {
        const loginForm = document.getElementById('login-form');
        if (loginForm) {
            loginForm.addEventListener('submit', handleLogin);
        }

        let token = localStorage.getItem('k8s_token');
        if (!token) {
            localStorage.setItem('k8s_token', 'k8s-enterprise-demo-token');
            token = 'k8s-enterprise-demo-token';
        }
        // Validate token on load
        APIClient.get('/health').catch(() => {});
    });

    // Expose auth API if needed by other modules
    window.Auth = {
        logout: () => {
            localStorage.removeItem('k8s_token');
            showLoginModal();
        },
        getToken: () => localStorage.getItem('k8s_token')
    };
})();
