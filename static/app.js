let ws = null;
let currentUser = null;
let currentSecret = null;
let gameState = null;
let lastStateJSON = '';
let connectedPlayers = [];
let refreshInterval = null;
let gameHistory = null;

// Cookie helpers
function setCookie(name, value, days = 365) {
    const date = new Date();
    date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
    const expires = "expires=" + date.toUTCString();
    document.cookie = name + "=" + value + ";" + expires + ";path=/";
}

function getCookie(name) {
    const nameEQ = name + "=";
    const ca = document.cookie.split(';');
    for (let i = 0; i < ca.length; i++) {
        let c = ca[i];
        while (c.charAt(0) === ' ') c = c.substring(1, c.length);
        if (c.indexOf(nameEQ) === 0) return c.substring(nameEQ.length, c.length);
    }
    return null;
}

function deleteCookie(name) {
    document.cookie = name + "=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
}

const loginScreen = document.getElementById('login-screen');
const gameScreen = document.getElementById('game-screen');
const loginUsernameInput = document.getElementById('login-username');
const loginSecretInput = document.getElementById('login-secret');
const signupUsernameInput = document.getElementById('signup-username');
const signupSecretInput = document.getElementById('signup-secret');
const loginBtn = document.getElementById('login-btn');
const signupBtn = document.getElementById('signup-btn');
const loginError = document.getElementById('login-error');
const signupError = document.getElementById('signup-error');
const userInfo = document.getElementById('user-info');
const phaseInfo = document.getElementById('phase-info');
const countriesDisplay = document.getElementById('countries-display');
const merchantsDisplay = document.getElementById('merchants-display');
const actionsList = document.getElementById('actions-list');
const queuedActionsList = document.getElementById('queued-actions-list');
const rejectedActionsList = document.getElementById('rejected-actions-list');
const adminPanel = document.getElementById('admin-panel');
const logoutBtn = document.getElementById('logout-btn');
const gameNameDisplay = document.getElementById('game-name-display');
const historyDisplay = document.getElementById('history-display');

loginBtn.addEventListener('click', login);
signupBtn.addEventListener('click', signup);
logoutBtn.addEventListener('click', logout);
loginUsernameInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') login();
});
loginSecretInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') login();
});
signupUsernameInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') signup();
});
signupSecretInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') signup();
});

document.getElementById('advance-btn').addEventListener('click', () => {
    const btn = document.getElementById('advance-btn');
    btn.disabled = true;
    send({ type: 'advance' });
    setTimeout(() => { btn.disabled = false; }, 5000);
});

document.getElementById('add-country-btn').addEventListener('click', () => {
    const countryId = document.getElementById('new-country-id').value.trim();
    const monarchId = document.getElementById('new-monarch-id').value;
    if (countryId && monarchId) {
        send({ type: 'add_country', country_id: countryId, monarch_id: monarchId });
        document.getElementById('new-country-id').value = '';
        document.getElementById('new-monarch-id').value = '';
    }
});

document.getElementById('add-merchant-btn').addEventListener('click', () => {
    const merchantId = document.getElementById('new-merchant-id').value;
    const countryId = document.getElementById('merchant-country-select').value;
    if (merchantId && countryId) {
        send({ type: 'add_merchant', player_id: merchantId, country_id: countryId });
        document.getElementById('new-merchant-id').value = '';
    }
});

async function signup() {
    const name = signupUsernameInput.value.trim();
    const secret = signupSecretInput.value.trim();

    if (!name || !secret) {
        signupError.textContent = 'Please enter username and secret';
        signupError.style.color = '#ff6b6b';
        return;
    }

    try {
        const response = await fetch('/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, secret })
        });

        if (response.ok) {
            signupError.textContent = '';
            // Save credentials to cookie
            setCookie('crown_user', name);
            setCookie('crown_secret', secret);
            // Auto-login after successful registration
            connectToServer(name, secret);
        } else {
            const text = await response.text();
            signupError.style.color = '#ff6b6b';
            signupError.textContent = text;
        }
    } catch (err) {
        signupError.style.color = '#ff6b6b';
        signupError.textContent = 'Connection error';
    }
}

async function login() {
    const name = loginUsernameInput.value.trim();
    const secret = loginSecretInput.value.trim();

    if (!name || !secret) {
        loginError.textContent = 'Please enter username and secret';
        loginError.style.color = '#ff6b6b';
        return;
    }

    try {
        const response = await fetch('/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, secret })
        });

        if (response.ok) {
            loginError.textContent = '';
            // Save credentials to cookie
            setCookie('crown_user', name);
            setCookie('crown_secret', secret);
            connectToServer(name, secret);
        } else {
            const text = await response.text();
            loginError.style.color = '#ff6b6b';
            loginError.textContent = text;
        }
    } catch (err) {
        loginError.style.color = '#ff6b6b';
        loginError.textContent = 'Connection error';
    }
}

function connectToServer(name, secret) {
    currentUser = name;
    currentSecret = secret;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

    ws.onopen = () => {
        loginScreen.classList.add('hidden');
        gameScreen.classList.remove('hidden');
        userInfo.textContent = `Logged in as: ${currentUser}`;

        if (currentUser === 'admin') {
            adminPanel.classList.remove('hidden');
            document.getElementById('game-content').style.gridTemplateColumns = '1fr 1fr 1fr 1fr';
        } else {
            document.getElementById('game-content').style.gridTemplateColumns = '1fr 1fr 1fr';
        }

        log('Connected to server', 'received');
        refreshState();
        refreshActions();
        refreshQueuedActions();
        refreshHistory();

        refreshInterval = setInterval(() => {
            refreshState();
            refreshActions();
            refreshQueuedActions();
            refreshHistory();
        }, 5000);
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        log('Received: ' + JSON.stringify(data, null, 2), data.success === false ? 'error' : 'received');

        // Handle connected_players broadcast
        if (data.type === 'connected_players') {
            connectedPlayers = (data.players || []).sort();
            renderConnectedPlayers();
            updateMonarchSelect();
            updateMerchantSelect();
            return;
        }

        // Handle history response
        if (data.type === 'history' || data.type === 'history_update') {
            if (data.history) {
                gameHistory = data.history;
                renderHistory(data.history);
            }
            return;
        }

        // Only re-render state if it actually changed
        if (data.state) {
            const newStateJSON = JSON.stringify(data.state);
            if (newStateJSON !== lastStateJSON) {
                lastStateJSON = newStateJSON;
                gameState = data.state;
                renderState(data.state);
                updateAdminSelects();
            }
        }

        if (data.actions !== undefined) {
            // Check if this is a queued actions response (has phase but no player_id)
            if (data.phase !== undefined && data.player_id === undefined && data.state === undefined) {
                renderQueuedActions(data.actions);
            } else {
                renderActions(data.actions);
            }
        }

        // Handle cancel actions response
        if (data.removed !== undefined) {
            refreshQueuedActions();
            refreshActions();
        }

        // Handle submit response (new format with single action)
        if (data.action !== undefined) {
            if (data.success === false && data.rejection_reason) {
                // Convert to old rejected_actions format for existing render function
                renderRejectedActions([{ action: data.action, reason: data.rejection_reason }]);
            } else if (data.success === true) {
                // Clear rejected actions on successful submit
                renderRejectedActions([]);
                // Refresh queued actions after successful submit
                refreshQueuedActions();
            }
        }
    };

    ws.onerror = (err) => {
        log('WebSocket error', 'error');
    };

    ws.onclose = () => {
        log('Disconnected from server', 'error');
        if (refreshInterval) {
            clearInterval(refreshInterval);
            refreshInterval = null;
        }
    };
}

function send(payload) {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        log('Not connected', 'error');
        return;
    }

    const message = {
        user: currentUser,
        secret: currentSecret,
        payload: payload
    };

    ws.send(JSON.stringify(message));
    log('Sent: ' + JSON.stringify(payload, null, 2), 'sent');
}

function log(message, type = 'received') {
    const prefix = type === 'sent' ? '→' : type === 'error' ? '✗' : '←';
    console.log(`${prefix} ${message}`);
}

function refreshState() {
    send({ type: 'get_state' });
}

function refreshActions() {
    if (currentUser && currentUser !== 'admin') {
        send({ type: 'get_actions', player_id: currentUser });
    }
}

function refreshQueuedActions() {
    if (currentUser === 'admin') {
        // Admin sees all queued actions
        send({ type: 'get_queued' });
    } else if (currentUser) {
        // Players see only their queued actions
        send({ type: 'get_queued', player_id: currentUser });
    }
}

function refreshHistory() {
    send({ type: 'get_history' });
}

function logout() {
    if (ws) {
        ws.close();
        ws = null;
    }
    if (refreshInterval) {
        clearInterval(refreshInterval);
        refreshInterval = null;
    }
    currentUser = null;
    currentSecret = null;
    gameState = null;
    lastStateJSON = '';
    connectedPlayers = [];
    gameHistory = null;

    // Clear cookies
    deleteCookie('crown_user');
    deleteCookie('crown_secret');

    gameScreen.classList.add('hidden');
    adminPanel.classList.add('hidden');
    loginScreen.classList.remove('hidden');

    loginUsernameInput.value = '';
    loginSecretInput.value = '';
    signupUsernameInput.value = '';
    signupSecretInput.value = '';
    loginError.textContent = '';
    loginError.style.color = '#ff6b6b';
    signupError.textContent = '';
    signupError.style.color = '#ff6b6b';

    countriesDisplay.innerHTML = '';
    merchantsDisplay.innerHTML = '';
    actionsList.innerHTML = '';
    queuedActionsList.innerHTML = '';
    historyDisplay.innerHTML = '';
    gameNameDisplay.textContent = '';
}

function renderState(state) {
    phaseInfo.textContent = `Round ${state.turn} - Phase ${formatPhase(state.phase)}`;

    countriesDisplay.innerHTML = '';
    for (const [id, country] of Object.entries(state.countries || {})) {
        const card = document.createElement('div');
        card.className = 'country-card';
        if (country.hp <= 0) card.classList.add('defeated');

        const status = country.is_republic ? 'Republic' : `Monarch: ${country.monarch_id}`;
        const healthPercent = Math.max(0, (country.hp / 10) * 100);

        card.innerHTML = `
            <div class="country-header">
                <span class="country-name">${country.country_id}</span>
                <span class="country-status">${status}</span>
            </div>
            <div class="health-bar">
                <div class="health-fill" style="width: ${healthPercent}%"></div>
                <span class="health-text">${country.hp} HP</span>
            </div>
            <div class="country-stats">
                <div class="stat">
                    <span class="stat-label">Gold</span>
                    <span class="stat-value">${country.gold}</span>
                </div>
                <div class="stat">
                    <span class="stat-label">Army</span>
                    <span class="stat-value">${country.army_strength}</span>
                </div>
                <div class="stat">
                    <span class="stat-label">Peasants</span>
                    <span class="stat-value">${country.peasants}</span>
                </div>
                <div class="stat">
                    <span class="stat-label">Revolt</span>
                    <span class="stat-value">${country.revolt_risk}/6</span>
                </div>
            </div>
        `;
        countriesDisplay.appendChild(card);
    }

    merchantsDisplay.innerHTML = '';
    for (const [id, merchant] of Object.entries(state.merchants || {})) {
        const card = document.createElement('div');
        card.className = 'merchant-card';

        card.innerHTML = `
            <div class="merchant-header">
                <span class="merchant-name">${merchant.player_id}</span>
                <span class="merchant-location">${merchant.country_id}</span>
            </div>
            <div class="merchant-stats">
                <div class="stat">
                    <span class="stat-label">Stored</span>
                    <span class="stat-value">${merchant.stored_gold}</span>
                </div>
                <div class="stat">
                    <span class="stat-label">Invested</span>
                    <span class="stat-value">${merchant.invested_gold}</span>
                </div>
            </div>
        `;
        merchantsDisplay.appendChild(card);
    }
}

function updateAdminSelects() {
    if (!gameState || currentUser !== 'admin') return;

    const countrySelect = document.getElementById('merchant-country-select');
    const prevCountry = countrySelect.value;
    countrySelect.innerHTML = '';
    for (const countryId of Object.keys(gameState.countries || {})) {
        const option = document.createElement('option');
        option.value = countryId;
        option.textContent = countryId;
        countrySelect.appendChild(option);
    }
    if (prevCountry) countrySelect.value = prevCountry;

    updateMerchantSelect();
}

function renderConnectedPlayers() {
    const list = document.getElementById('connected-players-list');
    if (!list) return;

    list.innerHTML = '';
    if (connectedPlayers.length === 0) {
        list.innerHTML = '<div class="no-players">No players connected</div>';
        return;
    }

    connectedPlayers.forEach(name => {
        const tag = document.createElement('span');
        tag.className = 'player-tag';
        tag.textContent = name;
        list.appendChild(tag);
    });
}

function updateMonarchSelect() {
    const select = document.getElementById('new-monarch-id');
    if (!select) return;

    const prevValue = select.value;
    select.innerHTML = '<option value="">Select Monarch...</option>';

    connectedPlayers.forEach(name => {
        const option = document.createElement('option');
        option.value = name;
        option.textContent = name;
        select.appendChild(option);
    });

    if (prevValue) select.value = prevValue;
}

function updateMerchantSelect() {
    const select = document.getElementById('new-merchant-id');
    if (!select) return;

    const prevValue = select.value;
    select.innerHTML = '<option value="">Select Merchant...</option>';

    // Get current monarchs
    const monarchs = new Set();
    if (gameState && gameState.countries) {
        for (const country of Object.values(gameState.countries)) {
            if (country.monarch_id && !country.is_republic) {
                monarchs.add(country.monarch_id);
            }
        }
    }

    // Get existing merchants
    const existingMerchants = new Set();
    if (gameState && gameState.merchants) {
        for (const merchant of Object.values(gameState.merchants)) {
            existingMerchants.add(merchant.player_id);
        }
    }

    // Filter available players
    connectedPlayers.forEach(name => {
        if (!monarchs.has(name) && !existingMerchants.has(name)) {
            const option = document.createElement('option');
            option.value = name;
            option.textContent = name;
            select.appendChild(option);
        }
    });

    if (prevValue) select.value = prevValue;
}

function formatPhase(phase) {
    const phases = {
        'taxation': 'Taxation',
        'negotiation': 'Negotiation',
        'spending': 'Spending',
        'war': 'War',
        'assessment': 'Assessment'
    };
    return phases[phase] || phase;
}

function parseAmountRange(value) {
    if (typeof value === 'string') {
        const match = value.match(/^<AMOUNT:(\d+)-(\d+)>$/);
        if (match) {
            return { min: parseInt(match[1]), max: parseInt(match[2]) };
        }
    }
    return null;
}

function getActionKey(action) {
    // Create a unique identifier for this action to preserve input values
    return `${action.type}_${action.player_id || ''}_${action.merchant_id || ''}_${action.country_id || ''}`;
}

function renderActions(actions) {
    // Save current input values before clearing
    const savedValues = {};
    actionsList.querySelectorAll('.amount-input').forEach(input => {
        const key = input.dataset.actionKey;
        if (key) {
            savedValues[key] = input.value;
        }
    });

    actionsList.innerHTML = '';

    if (!actions || actions.length === 0) {
        const empty = document.createElement('div');
        empty.textContent = 'No actions available';
        empty.style.color = '#666';
        actionsList.appendChild(empty);
        return;
    }

    actions = [...actions].sort((a, b) => formatActionLabel(a).localeCompare(formatActionLabel(b)));

    actions.forEach(action => {
        const range = parseAmountRange(action.amount);

        if (range) {
            const container = document.createElement('div');
            container.className = 'action-with-amount';

            const label = document.createElement('span');
            label.className = 'action-label';
            label.textContent = formatActionLabel(action);

            const input = document.createElement('input');
            input.type = 'number';
            input.min = range.min;
            input.max = range.max;
            input.className = 'amount-input';

            // Generate action key and store it on the input
            const actionKey = getActionKey(action);
            input.dataset.actionKey = actionKey;

            // Restore saved value if it exists and is valid, otherwise use max
            const savedValue = savedValues[actionKey];
            if (savedValue !== undefined) {
                const numValue = parseInt(savedValue);
                if (!isNaN(numValue) && numValue >= range.min && numValue <= range.max) {
                    input.value = numValue;
                } else {
                    input.value = range.max > 0 ? range.max : range.min;
                }
            } else {
                input.value = range.max > 0 ? range.max : range.min;
            }

            const btn = document.createElement('button');
            btn.textContent = 'Go';
            btn.addEventListener('click', () => {
                const amount = parseInt(input.value);
                if (!isNaN(amount) && amount >= range.min && amount <= range.max) {
                    const submitAction = { ...action, amount };
                    send({ type: 'submit', action: submitAction });
                } else {
                    renderRejectedActions([{
                        action: action,
                        reason: `Amount must be between ${range.min} and ${range.max}`
                    }]);
                }
            });

            container.appendChild(label);
            container.appendChild(input);
            container.appendChild(btn);
            actionsList.appendChild(container);
        } else {
            const btn = document.createElement('button');
            btn.textContent = formatAction(action);
            btn.addEventListener('click', () => {
                send({ type: 'submit', action: action });
            });
            actionsList.appendChild(btn);
        }
    });
}

function renderQueuedActions(actions) {
    queuedActionsList.innerHTML = '';

    if (!actions || actions.length === 0) {
        const empty = document.createElement('div');
        empty.textContent = 'No queued actions';
        empty.style.color = '#666';
        empty.style.fontSize = '0.9em';
        queuedActionsList.appendChild(empty);
        return;
    }

    actions.forEach(action => {
        const item = document.createElement('div');
        item.className = 'queued-action-item';
        item.style.padding = '8px';
        item.style.marginBottom = '4px';
        item.style.backgroundColor = '#2a2a2a';
        item.style.borderRadius = '4px';
        item.style.fontSize = '0.9em';

        const playerLabel = document.createElement('span');
        playerLabel.style.color = '#4ecdc4';
        playerLabel.style.fontWeight = 'bold';
        playerLabel.textContent = action.player_id + ': ';

        const actionText = document.createElement('span');
        actionText.style.color = '#e0e0e0';
        actionText.textContent = formatAction(action);

        item.appendChild(playerLabel);
        item.appendChild(actionText);
        queuedActionsList.appendChild(item);
    });

    // Show "Cancel All" button if the current user has queued actions
    const hasOwnActions = actions.some(a => a.player_id === currentUser);
    if (hasOwnActions && currentUser !== 'admin') {
        const cancelBtn = document.createElement('button');
        cancelBtn.textContent = 'Cancel All';
        cancelBtn.style.marginTop = '8px';
        cancelBtn.style.backgroundColor = '#cc3333';
        cancelBtn.style.color = '#fff';
        cancelBtn.style.border = 'none';
        cancelBtn.style.padding = '8px 16px';
        cancelBtn.style.borderRadius = '4px';
        cancelBtn.style.cursor = 'pointer';
        cancelBtn.style.width = '100%';
        cancelBtn.addEventListener('click', () => {
            send({ type: 'cancel_actions', player_id: currentUser });
        });
        queuedActionsList.appendChild(cancelBtn);
    }
}

function renderRejectedActions(rejectedActions) {
    if (!rejectedActionsList) return; // Element might not exist in older HTML

    const container = document.getElementById('rejected-actions-container');
    rejectedActionsList.innerHTML = '';

    if (!rejectedActions || rejectedActions.length === 0) {
        if (container) container.style.display = 'none';
        return;
    }

    if (container) container.style.display = 'block';

    rejectedActions.forEach(rejected => {
        const item = document.createElement('div');
        item.className = 'rejected-action-item';
        item.style.padding = '10px';
        item.style.marginBottom = '8px';
        item.style.backgroundColor = '#3a2020';
        item.style.borderLeft = '3px solid #ff4444';
        item.style.borderRadius = '4px';
        item.style.fontSize = '0.9em';

        const actionText = document.createElement('div');
        actionText.style.color = '#e0e0e0';
        actionText.style.marginBottom = '4px';
        actionText.textContent = formatAction(rejected.action);

        const reasonText = document.createElement('div');
        reasonText.style.color = '#ff6666';
        reasonText.style.fontSize = '0.85em';
        reasonText.textContent = '⚠ ' + rejected.reason;

        item.appendChild(actionText);
        item.appendChild(reasonText);
        rejectedActionsList.appendChild(item);
    });
}

function formatActionLabel(action) {
    switch (action.type) {
        case 'tax_merchants':
            return `Tax ${action.merchant_id}`;
        case 'merchant_invest':
            return 'Invest';
        case 'monarch_invest':
            return `Invest in ${action.merchant_id}`;
        case 'build_army':
            return 'Build Army';
        case 'contribute_army':
            return 'Contribute to Army';
        default:
            return formatAction(action);
    }
}

function formatAction(action) {
    switch (action.type) {
        case 'tax_peasants_low':
            return 'Tax Peasants (Low)';
        case 'tax_peasants_high':
            return 'Tax Peasants (High)';
        case 'tax_merchants':
            return `Tax ${action.merchant_id} (${action.amount})`;
        case 'build_army':
            return `Build Army (${action.amount})`;
        case 'merchant_invest':
            return `Invest ${action.amount}`;
        case 'monarch_invest':
            return `Invest ${action.amount} in ${action.merchant_id}`;
        case 'merchant_hide':
            return 'Hide Gold';
        case 'attack':
            return `Attack ${action.target_id}`;
        case 'no_attack':
            return 'No Attack';
        case 'remain':
            return 'Remain';
        case 'flee':
            return `Flee to ${action.target_id}`;
        case 'revolt':
            return 'Revolt!';
        case 'vote_tax_low':
            return 'Vote: Low Tax';
        case 'vote_tax_high':
            return 'Vote: High Tax';
        case 'contribute_army':
            return `Contribute ${action.amount} to Army`;
        case 'vote_attack':
            return `Vote: Attack ${action.target_id}`;
        case 'vote_no_attack':
            return 'Vote: No Attack';
        default:
            return action.type;
    }
}

function renderHistory(history) {
    if (!history) return;

    // Display game name
    if (history.game_name) {
        gameNameDisplay.textContent = `Game: ${history.game_name}`;
    }

    historyDisplay.innerHTML = '';

    // Group actions by phase
    const actionsByPhase = {};
    const actions = history.actions || [];

    actions.forEach(entry => {
        const key = `Turn ${entry.turn} - ${formatPhase(entry.phase)}`;
        if (!actionsByPhase[key]) {
            actionsByPhase[key] = [];
        }
        actionsByPhase[key].push(entry);
    });

    // Render actions grouped by phase
    Object.keys(actionsByPhase).forEach(phaseKey => {
        const group = document.createElement('div');
        group.className = 'history-phase-group';

        const header = document.createElement('div');
        header.className = 'history-phase-header';
        header.textContent = phaseKey;
        group.appendChild(header);

        actionsByPhase[phaseKey].forEach(entry => {
            const actionDiv = document.createElement('div');
            actionDiv.className = 'history-action-entry';

            const playerSpan = document.createElement('span');
            playerSpan.className = 'history-player';
            playerSpan.textContent = entry.player_id + ': ';

            const actionSpan = document.createElement('span');
            actionSpan.className = 'history-action';
            actionSpan.textContent = formatAction(entry.action);

            const timeSpan = document.createElement('span');
            timeSpan.className = 'history-time';
            if (entry.timestamp) {
                const time = new Date(entry.timestamp);
                timeSpan.textContent = time.toLocaleTimeString();
            }

            actionDiv.appendChild(playerSpan);
            actionDiv.appendChild(actionSpan);
            actionDiv.appendChild(timeSpan);
            group.appendChild(actionDiv);
        });

        historyDisplay.appendChild(group);
    });

    // Show message if no history
    if (actions.length === 0) {
        const empty = document.createElement('div');
        empty.textContent = 'No history yet';
        empty.style.color = '#666';
        empty.style.textAlign = 'center';
        empty.style.padding = '2rem';
        historyDisplay.appendChild(empty);
    }
}

// Auto-login on page load if credentials are saved
window.addEventListener('DOMContentLoaded', () => {
    const savedUser = getCookie('crown_user');
    const savedSecret = getCookie('crown_secret');

    if (savedUser && savedSecret) {
        // Attempt to auto-login
        loginUsernameInput.value = savedUser;
        loginSecretInput.value = savedSecret;
        login();
    }
});
