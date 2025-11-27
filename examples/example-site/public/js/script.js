// Forgejo Pages Example Site JavaScript

// Add smooth scrolling to all links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function (e) {
        e.preventDefault();
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
            target.scrollIntoView({
                behavior: 'smooth',
                block: 'start'
            });
        }
    });
});

// Add animation to feature cards on scroll
const observerOptions = {
    threshold: 0.1,
    rootMargin: '0px 0px -50px 0px'
};

const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            entry.target.style.opacity = '0';
            entry.target.style.transform = 'translateY(20px)';

            setTimeout(() => {
                entry.target.style.transition = 'opacity 0.5s ease, transform 0.5s ease';
                entry.target.style.opacity = '1';
                entry.target.style.transform = 'translateY(0)';
            }, 100);

            observer.unobserve(entry.target);
        }
    });
}, observerOptions);

// Observe all feature cards
document.querySelectorAll('.feature').forEach(feature => {
    observer.observe(feature);
});

// Log page load
console.log('Forgejo Pages site loaded successfully!');
console.log('Visit https://code.squarecows.com/SquareCows/pages-server for more information');

// Add current year to footer copyright
const yearElements = document.querySelectorAll('footer p');
yearElements.forEach(el => {
    if (el.textContent.includes('2025')) {
        el.textContent = el.textContent.replace('2025', new Date().getFullYear());
    }
});
