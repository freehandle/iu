// Timeline JavaScript functionality
document.addEventListener('DOMContentLoaded', function() {
    initializeTimeline();
    setupEventListeners();
    loadSamplePosts();
});

// Initialize timeline functionality
function initializeTimeline() {
    console.log('Timeline initialized');
}

// Setup event listeners
function setupEventListeners() {
    // Topic tag suggestions
    document.querySelectorAll('.topic-tag').forEach(tag => {
        tag.addEventListener('click', function() {
            const topic = this.getAttribute('data-topic');
            insertTopic(topic);
        });
    });

    // Filter buttons
    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            setActiveFilter(this);
        });
    });

    // Auto-resize textarea
    const textarea = document.getElementById('newPostContent');
    if (textarea) {
        textarea.addEventListener('input', autoResize);
    }
}

// Insert topic into textarea
function insertTopic(topic) {
    const textarea = document.getElementById('newPostContent');
    const cursorPos = textarea.selectionStart;
    const textBefore = textarea.value.substring(0, cursorPos);
    const textAfter = textarea.value.substring(cursorPos);
    
    // Check if we're at the beginning of a line or after a newline
    const isNewLine = cursorPos === 0 || textBefore.endsWith('\n');
    const insertText = isNewLine ? `${topic} ` : `\n${topic} `;
    
    textarea.value = textBefore + insertText + textAfter;
    textarea.selectionStart = textarea.selectionEnd = cursorPos + insertText.length;
    textarea.focus();
    
    // Trigger auto-resize
    autoResize.call(textarea);
}

// Auto-resize textarea
function autoResize() {
    this.style.height = 'auto';
    this.style.height = Math.min(this.scrollHeight, 200) + 'px';
}

// Set active filter
function setActiveFilter(activeBtn) {
    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    activeBtn.classList.add('active');
    
    const filter = activeBtn.getAttribute('data-filter');
    filterPosts(filter);
}

// Filter posts based on selected filter
function filterPosts(filter) {
    const posts = document.querySelectorAll('.post');
    
    posts.forEach(post => {
        if (filter === 'all') {
            post.style.display = 'block';
        } else if (filter === 'following') {
            // This would be implemented based on your following logic
            // For now, show all posts
            post.style.display = 'block';
        }
    });
}

// Submit new post
function submitPost() {
    const textarea = document.getElementById('newPostContent');
    const content = textarea.value.trim();
    
    if (!content) {
        showNotification('Por favor, escreva algo antes de postar!', 'error');
        return;
    }
    
    // Validate post format (check for topics)
    const topics = ['lendo', 'ouvindo', 'jogando', 'assistindo', 'comendo', 'preocupando', 'namorando', 'cobiçando'];
    const hasValidTopic = topics.some(topic => content.includes(topic + ':'));
    
    if (!hasValidTopic) {
        showNotification('Use pelo menos um tópico válido (ex: lendo:, ouvindo:, etc.)', 'warning');
        return;
    }
    
    // Create new post
    const newPost = createPostElement('rubis', new Date(), '', content);
    
    // Add to timeline
    const timeline = document.getElementById('timeline');
    timeline.insertBefore(newPost, timeline.firstChild);
    
    // Clear textarea
    textarea.value = '';
    textarea.style.height = 'auto';
    
    // Show success notification
    showNotification('Post criado com sucesso!', 'success');
    
    // Add animation
    newPost.style.opacity = '0';
    newPost.style.transform = 'translateY(-20px)';
    setTimeout(() => {
        newPost.style.transition = 'all 0.3s ease';
        newPost.style.opacity = '1';
        newPost.style.transform = 'translateY(0)';
    }, 10);
}

// Create post element
function createPostElement(quem, quando, onde, que) {
    const postDiv = document.createElement('div');
    postDiv.className = 'post';
    
    const formattedTime = formatTime(quando);
    const formattedContent = formatContent(que);
    
    postDiv.innerHTML = `
        <div class="post-header">
            <div class="quem">${quem}</div>
            <div class="quando">${formattedTime}</div>
            ${onde ? `<div class="onde">${onde}</div>` : ''}
        </div>
        <div class="que">
            ${formattedContent}
        </div>
        <div class="post-actions">
            <button class="action-btn" onclick="likePost(this)">
                <i class="far fa-heart"></i>
                <span>0</span>
            </button>
            <button class="action-btn" onclick="commentPost(this)">
                <i class="far fa-comment"></i>
                <span>0</span>
            </button>
            <button class="action-btn" onclick="sharePost(this)">
                <i class="far fa-share-square"></i>
            </button>
        </div>
    `;
    
    return postDiv;
}

// Format time for display
function formatTime(date) {
    const now = new Date();
    const diff = now - date;
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);
    
    if (minutes < 1) return 'Agora mesmo';
    if (minutes < 60) return `${minutes} min atrás`;
    if (hours < 24) return `${hours}h atrás`;
    if (days < 7) return `${days} dias atrás`;
    
    return date.toLocaleDateString('pt-BR');
}

// Format content with topic highlighting
function formatContent(content) {
    const lines = content.split('\n');
    const formattedLines = lines.map(line => {
        if (line.includes(':')) {
            const [topic, ...rest] = line.split(':');
            const description = rest.join(':').trim();
            return `<div class="linha"><strong>${topic}:</strong> ${description}</div>`;
        }
        return `<div class="linha">${line}</div>`;
    });
    
    return formattedLines.join('');
}

// Load sample posts for demonstration
function loadSamplePosts() {
    const samplePosts = [
        {
            quem: 'lari',
            quando: new Date(Date.now() - 300000), // 5 minutes ago
            onde: '',
            que: 'lendo:murder in mesopotamia\nouvindo:dupê\njogando:monstruosas'
        },
        {
            quem: 'maria',
            quando: new Date(Date.now() - 900000), // 15 minutes ago
            onde: 'São Paulo',
            que: 'assistindo:Stranger Things\ncomendo:pizza'
        },
        {
            quem: 'joão',
            quando: new Date(Date.now() - 1800000), // 30 minutes ago
            onde: '',
            que: 'preocupando:prova de amanhã\nouvindo:podcast sobre programação'
        }
    ];
    
    const timeline = document.getElementById('timeline');
    
    samplePosts.forEach(postData => {
        const postElement = createPostElement(
            postData.quem,
            postData.quando,
            postData.onde,
            postData.que
        );
        timeline.appendChild(postElement);
    });
}

// Post interaction functions
function likePost(button) {
    const icon = button.querySelector('i');
    const count = button.querySelector('span');
    
    if (icon.classList.contains('far')) {
        icon.classList.remove('far');
        icon.classList.add('fas');
        icon.style.color = '#e74c3c';
        count.textContent = parseInt(count.textContent) + 1;
    } else {
        icon.classList.remove('fas');
        icon.classList.add('far');
        icon.style.color = '';
        count.textContent = parseInt(count.textContent) - 1;
    }
}

function commentPost(button) {
    const count = button.querySelector('span');
    const currentCount = parseInt(count.textContent);
    
    // Simple comment simulation
    const comment = prompt('Digite seu comentário:');
    if (comment && comment.trim()) {
        count.textContent = currentCount + 1;
        showNotification('Comentário adicionado!', 'success');
    }
}

function sharePost(button) {
    // Simple share simulation
    if (navigator.share) {
        navigator.share({
            title: 'Vibe Post',
            text: 'Confira este post no Vibe!',
            url: window.location.href
        });
    } else {
        // Fallback for browsers that don't support Web Share API
        navigator.clipboard.writeText(window.location.href).then(() => {
            showNotification('Link copiado para a área de transferência!', 'success');
        });
    }
}

// Notification system
function showNotification(message, type = 'info') {
    // Remove existing notifications
    const existingNotifications = document.querySelectorAll('.notification');
    existingNotifications.forEach(notification => notification.remove());
    
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.innerHTML = `
        <div class="notification-content">
            <i class="fas fa-${getNotificationIcon(type)}"></i>
            <span>${message}</span>
        </div>
    `;
    
    // Add styles
    notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background: ${getNotificationColor(type)};
        color: white;
        padding: 15px 20px;
        border-radius: 10px;
        box-shadow: 0 4px 20px rgba(0,0,0,0.2);
        z-index: 1000;
        transform: translateX(100%);
        transition: transform 0.3s ease;
        max-width: 300px;
        font-weight: 500;
    `;
    
    document.body.appendChild(notification);
    
    // Animate in
    setTimeout(() => {
        notification.style.transform = 'translateX(0)';
    }, 10);
    
    // Auto remove after 3 seconds
    setTimeout(() => {
        notification.style.transform = 'translateX(100%)';
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 300);
    }, 3000);
}

function getNotificationIcon(type) {
    const icons = {
        success: 'check-circle',
        error: 'exclamation-circle',
        warning: 'exclamation-triangle',
        info: 'info-circle'
    };
    return icons[type] || 'info-circle';
}

function getNotificationColor(type) {
    const colors = {
        success: '#27ae60',
        error: '#e74c3c',
        warning: '#f39c12',
        info: '#3498db'
    };
    return colors[type] || '#3498db';
}

// Add CSS for post actions
const style = document.createElement('style');
style.textContent = `
    .post-actions {
        display: flex;
        gap: 15px;
        margin-top: 15px;
        padding-top: 15px;
        border-top: 1px solid #f0f0f0;
    }
    
    .action-btn {
        background: none;
        border: none;
        color: #666;
        cursor: pointer;
        padding: 8px 12px;
        border-radius: 20px;
        transition: all 0.3s ease;
        display: flex;
        align-items: center;
        gap: 5px;
        font-size: 0.9rem;
    }
    
    .action-btn:hover {
        background: #f8f9fa;
        color: #667eea;
    }
    
    .action-btn i {
        font-size: 1rem;
    }
    
    .notification-content {
        display: flex;
        align-items: center;
        gap: 10px;
    }
    
    .notification-content i {
        font-size: 1.2rem;
    }
`;
document.head.appendChild(style); 