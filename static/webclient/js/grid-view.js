// webclient/js/grid-view.js
const GridView = {
    container: null,
    items: [],

    init(containerSelector) {
        this.container = document.querySelector(containerSelector);
        if (!this.container) return;

        this.setupViewListener();
        this.setupDataTableHook();
        this.render();
    },

    setupDataTableHook() {
        const tryPopulate = () => {
            if (this.container && ViewToggle.getView() === 'grid') {
                this.populateFromDataTable();
            }
        };

        // Try immediately in case DataTable already initialized
        setTimeout(tryPopulate, 100);

        // Use MutationObserver to watch for row changes in the table body
        const tableBody = document.getElementById('file_manager_list_body');
        if (tableBody) {
            const observer = new MutationObserver(() => {
                tryPopulate();
            });
            observer.observe(tableBody, { childList: true, subtree: true });
        }

        // Also try periodically in case DataTable loads later
        const checkDt = setInterval(() => {
            tryPopulate();
        }, 500);

        // Stop checking after 10 seconds
        setTimeout(() => {
            clearInterval(checkDt);
            tryPopulate();
        }, 10000);
    },

    populateFromDataTable() {
        if (!this.container || ViewToggle.getView() !== 'grid') return;

        const rows = document.querySelectorAll('#file_manager_list_body tr');
        const items = [];

        rows.forEach(row => {
            const link = row.querySelector('a');
            if (!link) return;

            const name = link.textContent.trim();
            const href = link.getAttribute('href');
            const isDir = row.querySelector('.ki-folder') !== null;

            // Extract mtime from URL's &_= timestamp param
            let mtime = '';
            const mtimeMatch = href.match(/[?&]_=([^&]+)/);
            if (mtimeMatch) {
                mtime = mtimeMatch[1];
            }

            // Extract size from cell index 2 (e.g., "222.2 KiB")
            const cells = row.querySelectorAll('td');
            let size = '';
            if (cells.length >= 3) {
                size = cells[2].textContent.trim();
            }

            items.push({
                name: name,
                url: href,
                isDir: isDir,
                mtime: mtime,
                size: size,
                thumb_cache_key: ''
            });
        });

        if (items.length > 0) {
            this.setItems(items);
        }
    },

    setupViewListener() {
        document.addEventListener('viewChanged', (e) => {
            if (e.detail.view === 'grid') {
                this.show();
            } else {
                this.hide();
            }
        });
    },

    render() {
        if (ViewToggle.getView() === 'grid') {
            this.show();
        }
    },

    show() {
        if (this.container) {
            this.container.classList.remove('d-none');
        }
        // Hide the list container
        const listContainer = document.getElementById('file_manager_list_container');
        if (listContainer) {
            listContainer.classList.add('d-none');
        }
        this.populateFromDataTable();
        ThumbnailLoader.observeAll();
    },

    hide() {
        if (this.container) {
            this.container.classList.add('d-none');
        }
        // Show the list container
        const listContainer = document.getElementById('file_manager_list_container');
        if (listContainer) {
            listContainer.classList.remove('d-none');
        }
    },

    setItems(items) {
        this.items = items;
        this.renderItems();
    },

    renderItems() {
        if (!this.container || ViewToggle.getView() !== 'grid') return;

        this.container.innerHTML = this.items.map(item => this.createCellHTML(item)).join('');
        ThumbnailLoader.observeAll();
    },

    createCellHTML(item) {
        const filename = this.escapeHTML(item.name);
        const isImage = !item.isDir && this.isImageFile(item.name);
        const cacheKey = item.thumb_cache_key || '';
        const url = item.url || '';

        // Extract path from URL (e.g., "/web/client/files?path=/foo/bar&_=123")
        let path = '';
        const pathMatch = url.match(/[?&]path=([^&]+)/);
        if (pathMatch) {
            path = decodeURIComponent(pathMatch[1]);
        }

        const mtime = item.mtime || '';
        const size = item.size || '';

        if (item.isDir) {
            return `
                <a class="thumbnail-cell" href="${url}" data-filename="${filename}">
                    <div class="thumbnail-wrapper">
                        <div class="file-icon">
                            <i class="ki-duotone ki-folder fs-2"></i>
                        </div>
                    </div>
                    <span class="cell-filename">${filename}</span>
                </a>
            `;
        }

        if (!isImage) {
            return `
                <div class="thumbnail-cell" data-filename="${filename}" data-url="${url}">
                    <div class="thumbnail-wrapper">
                        <div class="file-icon">
                            <i class="ki-duotone ki-file fs-2"></i>
                        </div>
                    </div>
                    <span class="cell-filename">${filename}</span>
                </div>
            `;
        }

        return `
            <div class="thumbnail-cell" data-filename="${filename}" data-path="${path}" data-mtime="${mtime}" data-size="${size}" data-url="${url}">
                <div class="thumbnail-wrapper">
                    <div class="skeleton"></div>
                </div>
                <span class="cell-filename">${filename}</span>
            </div>
        `;
    },


    isImageFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].includes(ext);
    },

    escapeHTML(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};

// Auto-initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    GridView.init('.grid-view');
});