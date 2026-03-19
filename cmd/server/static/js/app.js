// State
const state = {
    top50: [],
    allSkills: [],
    categories: {},
    currentPage: 1,
    perPage: 12,
    totalSkills: 0,
    selectedCategory: '',
    searchQuery: ''
};

// DOM Elements
const top50List = document.getElementById('top50List');
const exploreList = document.getElementById('exploreList');
const pagination = document.getElementById('pagination');
const searchInput = document.getElementById('searchInput');
const categorySelect = document.getElementById('categorySelect');
const exploreTotal = document.getElementById('exploreTotal');
const totalSkills = document.getElementById('totalSkills');

// Format number
function formatNumber(num) {
    if (num >= 10000) {
        return (num / 10000).toFixed(1) + '万';
    }
    return num.toLocaleString();
}

// Create skill item (list style)
function createSkillItem(skill, showRank = false) {
    const item = document.createElement('div');
    item.className = 'skill-item';
    item.onclick = () => showSkillDetail(skill);

    item.innerHTML = `
        ${showRank ? `<span class="skill-rank">${skill.rank}</span>` : ''}
        <div class="skill-icon">${skill.icon || skill.name.charAt(0)}</div>
        <div class="skill-content">
            <div class="skill-header-row">
                <span class="skill-name">${skill.name}</span>
                <span class="skill-category">${skill.category}</span>
            </div>
            <p class="skill-description">${skill.description}</p>
        </div>
        <div class="skill-stats">
            <span class="skill-stat">
                <span class="skill-stat-icon">📥</span>
                ${formatNumber(skill.downloads)}
            </span>
            <span class="skill-stat">
                <span class="skill-stat-icon">⭐</span>
                ${skill.rating}
            </span>
        </div>
        <div class="skill-badges">
            ${skill.verified ? '<span class="skill-badge verified" title="官方认证">★</span>' : ''}
            ${skill.accelerated ? '<span class="skill-badge fast" title="加速下载">⚡</span>' : ''}
            ${skill.safe ? '<span class="skill-badge safe" title="安全审计">✓</span>' : ''}
        </div>
    `;

    return item;
}

// Show skill detail in modal
function showSkillDetail(skill) {
    const modal = document.getElementById('skillModal');
    const title = document.getElementById('skillModalTitle');
    const detail = document.getElementById('skillDetail');

    title.textContent = skill.name;
    detail.innerHTML = `
        <div class="skill-detail-icon">${skill.icon || skill.name.charAt(0)}</div>
        <h3>${skill.name}</h3>
        <span class="skill-category">${skill.category}</span>
        <p class="skill-description">${skill.description}</p>
        <div class="skill-detail-stats">
            <div class="skill-detail-stat">
                <span class="skill-detail-stat-value">${formatNumber(skill.downloads)}</span>
                <span class="skill-detail-stat-label">下载量</span>
            </div>
            <div class="skill-detail-stat">
                <span class="skill-detail-stat-value">${skill.rating}</span>
                <span class="skill-detail-stat-label">评分</span>
            </div>
        </div>
        <div class="skill-detail-actions">
            <button class="btn-download" onclick="downloadSkill(${skill.id})">
                📥 下载 Skill
            </button>
        </div>
    `;

    modal.classList.add('active');
}

function closeSkillModal() {
    document.getElementById('skillModal').classList.remove('active');
}

// Download skill
function downloadSkill(id) {
    window.location.href = `/api/skills/${id}/download`;
    showToast('开始下载...', 'success');
}

// Upload modal
function openUploadModal() {
    document.getElementById('uploadModal').classList.add('active');
    document.getElementById('uploadForm').reset();
    document.getElementById('fileName').textContent = '';
}

function closeUploadModal() {
    document.getElementById('uploadModal').classList.remove('active');
}

// Handle file selection
document.getElementById('skillFile')?.addEventListener('change', function(e) {
    const fileName = e.target.files[0]?.name;
    if (fileName) {
        document.getElementById('fileName').textContent = '已选择: ' + fileName;
    }
});

// Copy install code
function copyInstallCode() {
    const code = document.getElementById('installCode').textContent;
    navigator.clipboard.writeText(code).then(() => {
        showToast('已复制到剪贴板', 'success');
    }).catch(() => {
        showToast('复制失败', 'error');
    });
}

// Handle upload
async function handleUpload(e) {
    e.preventDefault();

    const submitBtn = document.getElementById('uploadSubmit');
    submitBtn.disabled = true;
    submitBtn.textContent = '上传中...';

    const formData = new FormData();
    formData.append('name', document.getElementById('skillName').value);
    formData.append('icon', document.getElementById('skillIcon').value);
    formData.append('category', document.getElementById('skillCategory').value);
    formData.append('description', document.getElementById('skillDescription').value);
    formData.append('file', document.getElementById('skillFile').files[0]);

    try {
        const response = await fetch('/api/skills/upload', {
            method: 'POST',
            body: formData
        });

        const result = await response.json();

        if (result.success) {
            showToast('上传成功！', 'success');
            closeUploadModal();
            // Refresh data
            loadTop50();
            loadAllSkills();
            loadCategories();
        } else {
            showToast(result.message || '上传失败', 'error');
        }
    } catch (error) {
        showToast('上传失败: ' + error.message, 'error');
    } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = '📤 上传';
    }
}

// Toast notification
function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = 'toast ' + type;
    toast.classList.add('show');

    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

// Render TOP 50
function renderTop50() {
    top50List.innerHTML = '';
    if (state.top50.length === 0) {
        top50List.innerHTML = '<div class="loading">暂无数据</div>';
        return;
    }
    state.top50.forEach(skill => {
        top50List.appendChild(createSkillItem(skill, true));
    });
}

// Render Explore
function renderExplore() {
    exploreList.innerHTML = '';
    if (state.allSkills.length === 0) {
        exploreList.innerHTML = '<div class="loading">暂无数据</div>';
        return;
    }
    state.allSkills.forEach(skill => {
        exploreList.appendChild(createSkillItem(skill, false));
    });
}

// Render pagination
function renderPagination() {
    const totalPages = Math.ceil(state.totalSkills / state.perPage);
    if (totalPages <= 1) {
        pagination.innerHTML = '';
        return;
    }

    let html = '';

    // Previous button
    html += `<button class="page-btn" onclick="changePage(${state.currentPage - 1})" ${state.currentPage === 1 ? 'disabled' : ''}>上一页</button>`;

    // Page numbers
    const start = Math.max(1, state.currentPage - 2);
    const end = Math.min(totalPages, state.currentPage + 2);

    if (start > 1) {
        html += `<button class="page-btn" onclick="changePage(1)">1</button>`;
        if (start > 2) html += `<span style="color: #9ca3af">...</span>`;
    }

    for (let i = start; i <= end; i++) {
        html += `<button class="page-btn ${i === state.currentPage ? 'active' : ''}" onclick="changePage(${i})">${i}</button>`;
    }

    if (end < totalPages) {
        if (end < totalPages - 1) html += `<span style="color: #9ca3af">...</span>`;
        html += `<button class="page-btn" onclick="changePage(${totalPages})">${totalPages}</button>`;
    }

    // Next button
    html += `<button class="page-btn" onclick="changePage(${state.currentPage + 1})" ${state.currentPage === totalPages ? 'disabled' : ''}>下一页</button>`;

    // Jump to page
    html += `
        <span style="color: #9ca3af; margin-left: 16px;">跳至</span>
        <input type="number" class="page-input" min="1" max="${totalPages}" value="${state.currentPage}"
               onchange="changePage(parseInt(this.value))" onkeypress="if(event.key==='Enter') changePage(parseInt(this.value))">
        <span style="color: #9ca3af;">页</span>
    `;

    pagination.innerHTML = html;
}

// Change page
function changePage(page) {
    const totalPages = Math.ceil(state.totalSkills / state.perPage);
    if (page < 1 || page > totalPages) return;
    state.currentPage = page;
    loadAllSkills();
}

// Load TOP 50
async function loadTop50() {
    try {
        top50List.innerHTML = '<div class="loading">加载中...</div>';
        const response = await fetch('/api/top50');
        const data = await response.json();
        state.top50 = data.skills || [];
        renderTop50();
    } catch (error) {
        top50List.innerHTML = '<div class="loading">加载失败</div>';
        console.error('Failed to load TOP 50:', error);
    }
}

// Load all skills
async function loadAllSkills() {
    try {
        exploreList.innerHTML = '<div class="loading">加载中...</div>';

        const params = new URLSearchParams({
            page: state.currentPage,
            per_page: state.perPage
        });

        if (state.selectedCategory) {
            params.append('category', state.selectedCategory);
        }
        if (state.searchQuery) {
            params.append('search', state.searchQuery);
        }

        const response = await fetch(`/api/skills?${params}`);
        const data = await response.json();
        state.allSkills = data.skills || [];
        state.totalSkills = data.total || 0;

        exploreTotal.textContent = formatNumber(data.total);
        renderExplore();
        renderPagination();
    } catch (error) {
        exploreList.innerHTML = '<div class="loading">加载失败</div>';
        console.error('Failed to load skills:', error);
    }
}

// Load categories
async function loadCategories() {
    try {
        const response = await fetch('/api/categories');
        const categories = await response.json();
        state.categories = categories;

        categorySelect.innerHTML = '<option value="">全部分类</option>';
        Object.keys(categories).sort().forEach(cat => {
            categorySelect.innerHTML += `<option value="${cat}">${cat} (${categories[cat]})</option>`;
        });
    } catch (error) {
        console.error('Failed to load categories:', error);
    }
}

// Load stats
async function loadStats() {
    try {
        const response = await fetch('/api/stats');
        const stats = await response.json();
        totalSkills.textContent = formatNumber(stats.total_skills);
    } catch (error) {
        console.error('Failed to load stats:', error);
    }
}

// Debounce function
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Event listeners
searchInput?.addEventListener('input', debounce((e) => {
    state.searchQuery = e.target.value;
    state.currentPage = 1;
    loadAllSkills();
}, 300));

categorySelect?.addEventListener('change', (e) => {
    state.selectedCategory = e.target.value;
    state.currentPage = 1;
    loadAllSkills();
});

// Close modal on outside click
document.querySelectorAll('.modal').forEach(modal => {
    modal.addEventListener('click', (e) => {
        if (e.target === modal) {
            modal.classList.remove('active');
        }
    });
});

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadTop50();
    loadAllSkills();
    loadCategories();
    loadStats();
});
