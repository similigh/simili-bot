// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/kavirubc
// Created: 2026-02-10
// Last Modified: 2026-02-10

document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('issueForm');
    const submitBtn = document.getElementById('submitBtn');
    const btnText = submitBtn.querySelector('.btn-text');
    const btnLoading = submitBtn.querySelector('.btn-loading');
    const resultsSection = document.getElementById('results');
    const errorSection = document.getElementById('error');
    const errorMessage = document.getElementById('errorMessage');

    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        // Get form values
        const title = document.getElementById('title').value.trim();
        const body = document.getElementById('body').value.trim();
        const org = document.getElementById('org').value.trim();
        const repo = document.getElementById('repo').value.trim();

        if (!title) {
            showError('Issue title is required');
            return;
        }

        // Show loading state
        setLoading(true);
        hideError();
        hideResults();

        try {
            const response = await fetch('/api/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    title: title,
                    body: body,
                    org: org || 'ballerina-platform',
                    repo: repo || 'ballerina-library',
                    labels: []
                })
            });

            const data = await response.json();

            if (!data.success) {
                showError(data.error || 'Analysis failed');
                return;
            }

            displayResults(data);
        } catch (err) {
            showError('Failed to connect to the server: ' + err.message);
        } finally {
            setLoading(false);
        }
    });

    function setLoading(loading) {
        submitBtn.disabled = loading;
        btnText.style.display = loading ? 'none' : 'inline';
        btnLoading.style.display = loading ? 'inline' : 'none';
    }

    function showError(message) {
        errorMessage.textContent = message;
        errorSection.style.display = 'block';
    }

    function hideError() {
        errorSection.style.display = 'none';
    }

    function hideResults() {
        resultsSection.style.display = 'none';
    }

    function displayResults(data) {
        resultsSection.style.display = 'block';

        // Display similar issues
        displaySimilarIssues(data.similar_issues || []);

        // Display duplicate check
        displayDuplicateCheck(data);

        // Display quality assessment
        displayQualityAssessment(data);

        // Display suggested labels
        displayLabels(data.suggested_labels || []);

        // Display transfer recommendation
        displayTransfer(data);
    }

    function displaySimilarIssues(issues) {
        const container = document.getElementById('similarIssues');

        if (!issues || issues.length === 0) {
            container.innerHTML = '<p class="no-results">No similar issues found in the database.</p>';
            return;
        }

        let html = '';
        issues.forEach(issue => {
            const score = (issue.Similarity * 100).toFixed(1);
            const stateClass = issue.State === 'open' ? 'status-open' : 'status-closed';
            const stateText = issue.State === 'open' ? 'Open' : 'Closed';

            html += `
                <div class="similar-issue">
                    <span class="similar-score">${score}%</span>
                    <div class="similar-info">
                        <div class="similar-title">
                            <a href="${issue.URL}" target="_blank">#${issue.Number}: ${escapeHtml(issue.Title)}</a>
                            <span class="status-badge ${stateClass}">${stateText}</span>
                        </div>
                        <div class="similar-meta">
                            Similarity: ${score}%
                        </div>
                    </div>
                </div>
            `;
        });

        container.innerHTML = html;
    }

    function displayDuplicateCheck(data) {
        const container = document.getElementById('duplicateResult');

        if (data.is_duplicate) {
            container.innerHTML = `
                <div class="duplicate-yes">
                    <p class="status">Potential Duplicate Detected</p>
                    <p class="reason">This issue appears to be a duplicate of <strong>#${data.duplicate_of}</strong></p>
                    ${data.duplicate_reason ? `<p class="reason">${escapeHtml(data.duplicate_reason)}</p>` : ''}
                </div>
            `;
        } else {
            container.innerHTML = `
                <div class="duplicate-no">
                    <p class="status">Not a Duplicate</p>
                    ${data.duplicate_reason ? `<p class="reason">${escapeHtml(data.duplicate_reason)}</p>` : '<p class="reason">This issue does not appear to be a duplicate of any existing issue.</p>'}
                </div>
            `;
        }
    }

    function displayQualityAssessment(data) {
        const container = document.getElementById('qualityResult');
        const score = data.quality_score || 0;
        const percentage = Math.round(score * 100);

        let scoreClass = 'score-low';
        let scoreLabel = 'Needs Improvement';
        if (percentage >= 80) {
            scoreClass = 'score-high';
            scoreLabel = 'Excellent';
        } else if (percentage >= 60) {
            scoreClass = 'score-medium';
            scoreLabel = 'Good';
        }

        let html = `
            <div class="quality-score">
                <div class="score-circle ${scoreClass}">${percentage}%</div>
                <div>
                    <strong>${scoreLabel}</strong>
                    <p style="color: #888; font-size: 0.9rem;">Quality Score</p>
                </div>
            </div>
        `;

        if (data.quality_issues && data.quality_issues.length > 0) {
            html += '<div class="quality-issues">';
            data.quality_issues.forEach(issue => {
                html += `<span class="quality-issue">${escapeHtml(issue)}</span>`;
            });
            html += '</div>';
        }

        container.innerHTML = html;
    }

    function displayLabels(labels) {
        const container = document.getElementById('labelsResult');

        if (!labels || labels.length === 0) {
            container.innerHTML = '<p class="no-results">No labels suggested.</p>';
            return;
        }

        let html = '';
        labels.forEach(label => {
            html += `<span class="label-tag">${escapeHtml(label)}</span>`;
        });

        container.innerHTML = html;
    }

    function displayTransfer(data) {
        const transferCard = document.getElementById('transferCard');
        const container = document.getElementById('transferResult');

        if (!data.transfer_target) {
            transferCard.style.display = 'none';
            return;
        }

        transferCard.style.display = 'block';
        container.innerHTML = `
            <div class="duplicate-yes" style="border-color: #eab308; border-left-color: #eab308;">
                <p class="status" style="color: #eab308;">Transfer Recommended</p>
                <p class="reason">This issue should be transferred to: <strong>${escapeHtml(data.transfer_target)}</strong></p>
                ${data.transfer_reason ? `<p class="reason">${escapeHtml(data.transfer_reason)}</p>` : ''}
            </div>
        `;
    }

    function escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
});
