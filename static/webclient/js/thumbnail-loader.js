// webclient/js/thumbnail-loader.js
const ThumbnailLoader = {
    observer: null,
    thumbURL: '/web/client/thumb',

    init() {
        this.setupIntersectionObserver();
    },

    setupIntersectionObserver() {
        this.observer = new IntersectionObserver(
            (entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        this.loadThumbnail(entry.target);
                        this.observer.unobserve(entry.target);
                    }
                });
            },
            { rootMargin: '200px' }
        );
    },

    observeAll() {
        const cells = document.querySelectorAll('.thumbnail-cell[data-path]');
        cells.forEach(cell => {
            const wrapper = cell.querySelector('.thumbnail-wrapper');
            if (wrapper && !wrapper.querySelector('img[src]') && !wrapper.querySelector('.error-state')) {
                this.observer.observe(cell);
            }
        });
    },

    loadThumbnail(cell) {
        const path = cell.dataset.path;
        const mtime = cell.dataset.mtime;
        const size = cell.dataset.size;
        const cacheKey = cell.dataset.cacheKey;

        // Non-image cells have file-icon instead of thumbnail-wrapper
        const wrapper = cell.querySelector('.thumbnail-wrapper');
        if (!wrapper) return;

        // Skip cells without path (directories have file-icon, not images)
        if (!path) return;

        // Build thumbnail URL with query params
        const url = `${this.thumbURL}?path=${encodeURIComponent(path)}&mtime=${mtime}&size=${size}`;
        this.showImage(cell, url);
    },

    showImage(cell, src) {
        const wrapper = cell.querySelector('.thumbnail-wrapper');
        if (!wrapper) return;

        const skeleton = wrapper.querySelector('.skeleton');
        if (skeleton) skeleton.remove();

        let img = wrapper.querySelector('img');
        if (!img) {
            img = document.createElement('img');
            img.alt = cell.dataset.filename || 'Thumbnail';
            wrapper.appendChild(img);
        }

        img.onload = () => cell.classList.add('loaded');
        img.onerror = () => this.showError(cell, 'Failed');
        img.src = src;
    },

    showError(cell, message) {
        const wrapper = cell.querySelector('.thumbnail-wrapper');
        if (!wrapper) return;

        const skeleton = wrapper.querySelector('.skeleton');
        if (skeleton) skeleton.remove();

        wrapper.innerHTML = `
            <div class="error-state">
                <i class="ki-duotone ki-picture fs-1"></i>
            </div>
        `;
    }
};

document.addEventListener('DOMContentLoaded', () => ThumbnailLoader.init());