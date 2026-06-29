// theme.js - Handles Dark/Light theme toggling
(function() {
    document.addEventListener("DOMContentLoaded", () => {
        const toggleBtn = document.getElementById('theme-toggle-btn');
        const root = document.documentElement;
        
        // Load preference from localStorage or fallback to dark (default)
        const savedTheme = localStorage.getItem('k8s_theme') || 'dark';
        applyTheme(savedTheme);

        if (toggleBtn) {
            toggleBtn.addEventListener('click', () => {
                const currentTheme = root.getAttribute('data-theme') || 'dark';
                const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
                applyTheme(newTheme);
            });
        }

        function applyTheme(theme) {
            root.setAttribute('data-theme', theme);
            localStorage.setItem('k8s_theme', theme);
            if (toggleBtn) {
                toggleBtn.textContent = theme === 'dark' ? '☀️ Light Mode' : '🌙 Dark Mode';
            }
        }
    });
})();
