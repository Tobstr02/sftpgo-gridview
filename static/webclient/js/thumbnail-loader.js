const ThumbnailLoader = {
    observer: null,
    thumbURL: '/web/client/thumb',
    maxRetries: 3,

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
        const cells = document.querySelectorAll('.thumbnail-cell[data-mtime]');
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

        const wrapper = cell.querySelector('.thumbnail-wrapper');
        if (!wrapper) return;

        if (!path) return;

        const url = `${this.thumbURL}?path=${encodeURIComponent(path)}&mtime=${mtime}&size=${size}`;
        this.fetchThumbnail(cell, url);
    },

    fetchThumbnail(cell, url) {
        fetch(url, { credentials: 'include' })
            .then(response => {
                if (response.status === 429) {
                    this.scheduleRetry(cell);
                    return;
                }
                if (!response.ok) {
                    this.showError(cell);
                    return;
                }
                response.blob().then(blob => {
                    const blobUrl = URL.createObjectURL(blob);
                    this.displayImage(cell, blobUrl);
                }).catch(() => {
                    this.showError(cell);
                });
            })
            .catch(() => {
                this.showError(cell);
            });
    },

    scheduleRetry(cell) {
        const retryCount = parseInt(cell.dataset.retryCount || '0', 10);
        if (retryCount >= this.maxRetries) {
            this.showError(cell);
            return;
        }
        cell.dataset.retryCount = retryCount + 1;
        const delay = 2000 + Math.random() * 5000;
        setTimeout(() => {
            const wrapper = cell.querySelector('.thumbnail-wrapper');
            if (wrapper && !wrapper.querySelector('img[src]')) {
                this.fetchThumbnail(cell, cell.dataset.currentUrl);
            }
        }, delay);
    },

    displayImage(cell, src) {
        const wrapper = cell.querySelector('.thumbnail-wrapper');
        if (!wrapper) return;

        const skeleton = wrapper.querySelector('.skeleton');
        if (skeleton) skeleton.remove();

        let img = wrapper.querySelector('img');
        if (!img) {
            img = document.createElement('img');
            img.alt = 'Loading thumbnail...';
            wrapper.appendChild(img);
        }

        img.onload = () => {
            img.alt = cell.dataset.filename || 'Thumbnail';
            cell.classList.add('loaded');
        };
        img.onerror = () => this.showError(cell);
        img.src = src;
        cell.dataset.currentUrl = src;
    },

    showError(cell) {
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