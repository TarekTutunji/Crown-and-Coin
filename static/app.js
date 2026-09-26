let ws = null;
let currentUser = null;
let currentSecret = null;
let gameState = null;
let lastStateJSON = '';
let connectedPlayers = [];
let readyPlayers = []; // Players who are done with this phase
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

document.getElementById('ready-btn').addEventListener('click', () => {
    send({ type: 'set_ready', player_id: currentUser, ready: !readyPlayers.includes(currentUser) });
});

document.getElementById('save-settings-btn').addEventListener('click', () => {
    const percent = parseInt(document.getElementById('investment-return').value, 10);
    if (isNaN(percent)) {
        showSettingsNote('Enter a whole number for the investment return', true);
        return;
    }
    settingsSaving = true;
    send({
        type: 'set_settings',
        investment_return_percent: percent,
        open_game: document.getElementById('open-game').checked
    });
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

let pendingAssign = null;
let settingsSaving = false; // Waiting for the answer to a settings change

document.getElementById('assign-role-btn').addEventListener('click', () => {
    const playerId = document.getElementById('assign-player-id').value;
    const role = document.getElementById('assign-role').value;
    const countryId = role === 'none' ? '' : document.getElementById('assign-country-select').value;
    if (!playerId || (role !== 'none' && !countryId)) {
        showAssignNote('Pick a player and a country first', true);
        return;
    }
    pendingAssign = { playerId, role, countryId };
    send({ type: 'assign_role', player_id: playerId, role: role, country_id: countryId });
});

// Starting a new game wipes the one in progress, so it takes two clicks
document.getElementById('new-game-btn').addEventListener('click', () => {
    document.getElementById('new-game-confirm').classList.remove('hidden');
    showNewGameNote('', false);
});

document.getElementById('new-game-cancel').addEventListener('click', () => {
    document.getElementById('new-game-confirm').classList.add('hidden');
});

document.getElementById('new-game-yes').addEventListener('click', () => {
    document.getElementById('new-game-confirm').classList.add('hidden');
    send({ type: 'new_game' });
});

// Calling the game over reveals every merchant's hidden gold on the projector
// board, so it takes two clicks as well. It changes nothing in the game, and
// the same section puts the live board back.
document.getElementById('end-game-btn').addEventListener('click', () => {
    document.getElementById('end-game-confirm').classList.remove('hidden');
    showEndGameNote('', false);
});

document.getElementById('end-game-cancel').addEventListener('click', () => {
    document.getElementById('end-game-confirm').classList.add('hidden');
});

document.getElementById('end-game-yes').addEventListener('click', () => {
    document.getElementById('end-game-confirm').classList.add('hidden');
    send({ type: 'end_game', ended: true });
});

document.getElementById('end-game-back').addEventListener('click', () => {
    send({ type: 'end_game', ended: false });
});

function showEndGameNote(text, isError) {
    const note = document.getElementById('end-game-note');
    note.textContent = text;
    note.className = isError ? 'admin-note error' : 'admin-note';
}

// Whether the summary is up is the board's business, not the engine's, so the
// panel reads it off the board itself. That also gets it right after a reload.
function renderEndGame(ended) {
    document.getElementById('end-game-btn').classList.toggle('hidden', ended);
    document.getElementById('end-game-back').classList.toggle('hidden', !ended);
    if (!ended) document.getElementById('end-game-confirm').classList.add('hidden');
}

async function refreshEndGame() {
    if (currentUser !== 'admin') return;
    try {
        const response = await fetch('board.json', { cache: 'no-store' });
        if (!response.ok) return;
        renderEndGame(!!(await response.json()).final);
    } catch (err) {
        // The board is the game leader's own screen; if it cannot be read, the
        // panel just keeps showing what it last knew
    }
}

function showNewGameNote(text, isError) {
    const note = document.getElementById('new-game-note');
    note.textContent = text;
    note.className = isError ? 'admin-note error' : 'admin-note';
}

document.getElementById('assign-role').addEventListener('change', () => {
    const role = document.getElementById('assign-role').value;
    document.getElementById('assign-country-select').classList.toggle('hidden', role === 'none');
});

function showAssignNote(text, isError) {
    const note = document.getElementById('assign-role-note');
    note.textContent = text;
    note.classList.toggle('error', isError);
}

function describeRole({ playerId, role, countryId }) {
    if (role === 'none') return `${playerId} was removed from the game`;
    return `${playerId} is now ${role === 'monarch' ? 'monarch' : 'a merchant'} of ${countryId}`;
}

async function signup() {
    const name = signupUsernameInput.value.trim();
    const secret = signupSecretInput.value.trim();

    if (!name || !secret) {
        signupError.textContent = 'Please enter username and secret';
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
            signupError.textContent = text;
        }
    } catch (err) {
        signupError.textContent = 'Connection error';
    }
}

async function login() {
    const name = loginUsernameInput.value.trim();
    const secret = loginSecretInput.value.trim();

    if (!name || !secret) {
        loginError.textContent = 'Please enter username and secret';
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
            loginError.textContent = text;
        }
    } catch (err) {
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
        userInfo.textContent = currentUser;

        const isAdmin = currentUser === 'admin';
        gameScreen.classList.toggle('admin-view', isAdmin);
        gameScreen.classList.toggle('player-view', !isAdmin);
        if (isAdmin) {
            adminPanel.classList.remove('hidden');
            document.getElementById('admin-bar').classList.remove('hidden');
            // The admin has no moves to make, so this panel shows what the
            // last phase did instead
            document.getElementById('actions-title').textContent = 'Last Phase Results';
            document.getElementById('queued-title').textContent = 'Queued Moves (all players)';
            refreshEndGame();
        } else {
            document.getElementById('actions-title').textContent = 'Your Moves';
            document.getElementById('queued-title').textContent = 'Queued Moves';
            document.getElementById('ready-container').classList.remove('hidden');
        }

        log('Connected to server', 'received');
        refreshState();
        refreshActions();
        refreshQueuedActions();
        refreshHistory();
        refreshSettings();

        refreshInterval = setInterval(() => {
            refreshState();
            refreshActions();
            refreshQueuedActions();
            refreshHistory();
            refreshSettings();
            // Resolving a phase puts the live board back, so the panel checks
            // rather than assuming the summary is still up
            refreshEndGame();
        }, 5000);
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        log('Received: ' + JSON.stringify(data, null, 2), data.success === false ? 'error' : 'received');

        // Handle connected_players broadcast
        if (data.type === 'connected_players') {
            connectedPlayers = (data.players || []).sort();
            readyPlayers = data.ready || [];
            renderConnectedPlayers();
            renderReadyButton();
            updateMonarchSelect();
            updateMerchantSelect();
            updateAssignSelects();
            return;
        }

        // The game has been called over, or the summary dismissed
        if (data.type === 'end_game') {
            if (data.success) {
                renderEndGame(!!data.ended);
                showEndGameNote(data.ended
                    ? 'The board is showing the summary. Step through it with Next, or put the live board back here.'
                    : 'The board is live again.', false);
            } else {
                showEndGameNote(data.error || 'Could not change the board', true);
            }
            return;
        }

        // A new game has been started: drop everything from the old one
        if (data.type === 'new_game') {
            if (data.success) {
                showNewGameNote(`New game started: ${data.game_name}`, false);
                renderEndGame(false); // A new game leaves no summary to show
                lastStateJSON = null;
                gameHistory = null;
                renderRejectedActions([]);
            } else {
                showNewGameNote(data.error || 'Could not start a new game', true);
            }
            refreshState();
            refreshActions();
            refreshQueuedActions();
            refreshHistory();
            return;
        }

        // Handle the game settings
        if (data.type === 'settings') {
            renderSettings(data);
            return;
        }

        // Handle history response
        if (data.type === 'history' || data.type === 'history_update') {
            if (data.history) {
                gameHistory = data.history;
                renderHistory(data.history);
                if (currentUser === 'admin') {
                    renderWarReport(data.history);
                    renderVoteReport(data.history);
                    renderLastResults(data.history);
                }
            }
            return;
        }

        // Handle the answer to a role change (it only carries success/error)
        if (pendingAssign && data.success !== undefined && Object.keys(data).every(k => k === 'success' || k === 'error')) {
            if (data.success) {
                showAssignNote(describeRole(pendingAssign), false);
                refreshState();
            } else {
                showAssignNote(data.error || 'Could not change role', true);
            }
            pendingAssign = null;
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

function refreshSettings() {
    send({ type: 'get_settings' });
}

// Shows the current settings to everyone, and keeps the admin's settings form
// in step with them (unless the admin is in the middle of editing it)
function renderSettings(data) {
    const settings = data.settings || {};
    const view = settings.open_game ? 'Open game: everyone sees every move' : 'Limited view';
    document.getElementById('settings-info').textContent =
        `Investments pay ${settings.investment_return_percent}% · ${view}`;

    if (settingsSaving) {
        settingsSaving = false;
        if (data.success) {
            showSettingsNote('Settings saved', false);
            lastStateJSON = null; // The view may have changed: redraw everything
            refreshState();
            refreshHistory();
        } else {
            showSettingsNote(data.error || 'Could not save the settings', true);
        }
    }

    const editing = ['investment-return', 'open-game'].includes(document.activeElement && document.activeElement.id);
    if (currentUser === 'admin' && !editing) {
        document.getElementById('investment-return').value = settings.investment_return_percent;
        document.getElementById('open-game').checked = !!settings.open_game;
    }
}

function showSettingsNote(text, isError) {
    const note = document.getElementById('settings-note');
    note.textContent = text;
    note.className = isError ? 'admin-note error' : 'admin-note';
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
    readyPlayers = [];
    gameHistory = null;

    // Clear cookies
    deleteCookie('crown_user');
    deleteCookie('crown_secret');

    gameScreen.classList.add('hidden');
    gameScreen.classList.remove('admin-view', 'player-view');
    adminPanel.classList.add('hidden');
    document.getElementById('admin-bar').classList.add('hidden');
    document.getElementById('my-panel').classList.add('hidden');
    loginScreen.classList.remove('hidden');
    document.getElementById('actions-title').textContent = 'Your Moves';
    document.getElementById('ready-container').classList.add('hidden');
    historyOpen = {};

    loginUsernameInput.value = '';
    loginSecretInput.value = '';
    signupUsernameInput.value = '';
    signupSecretInput.value = '';
    loginError.textContent = '';
    signupError.textContent = '';

    countriesDisplay.innerHTML = '';
    merchantsDisplay.innerHTML = '';
    actionsList.innerHTML = '';
    queuedActionsList.innerHTML = '';
    historyDisplay.innerHTML = '';
    gameNameDisplay.textContent = '';
}

const MAX_HP = 10; // Every country starts with 10 HP
let historyOpen = {}; // Chronicle phases the viewer opened or closed by hand

// Makes text safe to put inside HTML (player and country names are typed in)
function esc(value) {
    return String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}

// Crown for monarchies, hall for republics, coin for merchants
function emblem(kind) {
    return `<img class="emblem" src="emblems/${kind}.svg" alt="">`;
}

// Lights up the current phase in the header
function renderPhaseTrack(phase) {
    const steps = [...document.querySelectorAll('#phase-track li')];
    const at = steps.findIndex(li => li.dataset.phase === phase);
    steps.forEach((li, i) => {
        li.classList.toggle('current', i === at);
        li.classList.toggle('done', at >= 0 && i < at);
    });
}

function renderState(state) {
    phaseInfo.textContent = `Round ${state.turn}`;
    renderPhaseTrack(state.phase);
    renderMyPanel(state);
    renderCountries(state);
    renderMerchants(state);
}

// Where the current player stands: the country they rule, or their merchant
function findMyRole(state) {
    const ruled = Object.values(state.countries || {}).find(c => c.monarch_id === currentUser && !c.is_republic);
    const merchant = Object.values(state.merchants || {}).find(m => m.player_id === currentUser);
    if (ruled && (ruled.hp > 0 || !merchant)) return { kind: 'monarch', country: ruled };
    if (merchant) return { kind: 'merchant', merchant, country: (state.countries || {})[merchant.country_id] };
    return { kind: 'none' };
}

function statTile(label, value, extraClass = '') {
    return `<div class="my-stat"><div class="my-stat-label">${label}</div><div class="my-stat-value ${extraClass}">${value}</div></div>`;
}

// The player's own numbers, large, at the top of their moves column
function renderMyPanel(state) {
    const panel = document.getElementById('my-panel');
    if (currentUser === 'admin') {
        panel.classList.add('hidden');
        return;
    }
    panel.classList.remove('hidden');

    const role = findMyRole(state);
    if (role.kind === 'monarch') {
        const c = role.country;
        const secret = value => c.hidden ? '?' : value;
        let note = '';
        if (c.hp <= 0) {
            note = '<div class="my-note used">Your country has fallen.</div>';
        } else if (c.died_once) {
            note = '<div class="my-note used">Your country already came back from defeat once. The next defeat is final.</div>';
        } else {
            note = '<div class="my-note">If your country is defeated, it comes back once with 1 HP.</div>';
        }
        panel.innerHTML = `
            <div class="my-head">
                ${emblem('crown')}
                <div>
                    <div class="my-role monarch">Monarch</div>
                    <div class="my-name">${esc(currentUser)} of ${esc(c.country_id)}</div>
                </div>
            </div>
            <div class="my-stats">
                ${statTile('HP', `${Math.max(0, c.hp)}/${MAX_HP}`)}
                ${statTile('Treasury', secret(c.gold), 'gold')}
                ${statTile('Army', c.army_strength)}
                ${statTile('Peasants', secret(c.peasants))}
                ${statTile('Revolt', c.hidden ? '?' : `${c.revolt_risk}/6`)}
            </div>
            ${note}
        `;
    } else if (role.kind === 'merchant') {
        const m = role.merchant;
        const inRepublic = role.country && role.country.is_republic;
        const place = m.arriving ? `, bound for ${esc(m.country_id)}` : ` of ${esc(m.country_id)}`;
        panel.innerHTML = `
            <div class="my-head">
                ${emblem('coin')}
                <div>
                    <div class="my-role ${inRepublic ? 'republic' : 'merchant'}">${inRepublic ? 'Merchant of a republic' : 'Merchant'}</div>
                    <div class="my-name">${esc(currentUser)}${place}</div>
                </div>
            </div>
            <div class="my-stats">
                ${statTile('Purse', m.purse_hidden ? '?' : m.stored_gold, 'gold')}
                ${statTile('Hidden', m.hidden ? '?' : (m.hidden_gold || 0))}
                ${statTile('Invested', m.hidden ? '?' : m.invested_gold, 'gold')}
            </div>
            ${m.arriving ? '<div class="my-note">You join your new country at the start of the next round.</div>' : ''}
        `;
    } else {
        panel.innerHTML = `
            <div class="my-head">
                <div>
                    <div class="my-role none">Awaiting a role</div>
                    <div class="my-name">${esc(currentUser)}</div>
                </div>
            </div>
            <div class="my-note">The admin will make you a monarch or a merchant.</div>
        `;
    }
}

// One compact row per country, with the player's own country first
function renderCountries(state) {
    const role = currentUser === 'admin' ? { kind: 'none' } : findMyRole(state);
    const mine = role.country ? role.country.country_id : null;
    const mineTag = `<span class="you-tag">${role.kind === 'merchant' ? 'Home' : 'You'}</span>`;
    const countries = Object.values(state.countries || {}).sort((a, b) =>
        (b.country_id === mine) - (a.country_id === mine) ||
        (b.hp > 0) - (a.hp > 0) ||
        a.country_id.localeCompare(b.country_id));

    if (countries.length === 0) {
        countriesDisplay.innerHTML = '<div class="empty-note">No realms yet. The admin adds them.</div>';
        return;
    }

    let anyHidden = false;
    const hiddenMark = '<span class="secret">?</span>';
    const rows = countries.map(c => {
        const secret = value => c.hidden ? hiddenMark : value;
        if (c.hidden) anyHidden = true;
        const fallen = c.hp <= 0;
        const isYou = c.country_id === mine;
        const ruler = fallen ? 'Fallen'
            : c.is_republic ? 'Republic'
            : c.monarch_id ? `Monarch ${esc(c.monarch_id)}` : 'No monarch';
        const life = fallen ? ''
            : c.died_once
                ? '<span class="life-mark used" title="Already came back from defeat once. The next defeat is final.">♡</span>'
                : '<span class="life-mark" title="Comes back with 1 HP the first time it is defeated.">♥</span>';
        const hpPercent = Math.max(0, Math.min(100, (c.hp / MAX_HP) * 100));
        return `
            <tr class="${isYou ? 'is-you' : ''} ${fallen ? 'defeated' : ''}">
                <td>
                    <div class="realm-name-cell">
                        ${emblem(c.is_republic ? 'hall' : 'crown')}
                        <div>
                            <div class="realm-name">${esc(c.country_id)}${isYou ? mineTag : ''}</div>
                            <div class="realm-sub">${ruler}</div>
                        </div>
                    </div>
                </td>
                <td class="hp-cell">
                    ${life}${Math.max(0, c.hp)}
                    <div class="hp-bar"><div class="hp-fill ${c.hp <= 3 ? 'low' : ''}" style="width: ${hpPercent}%"></div></div>
                </td>
                <td class="gold">${secret(c.gold)}</td>
                <td ${c.hidden ? 'title="Army size as of the last war"' : ''}>${c.army_strength}${c.hidden ? '*' : ''}</td>
                <td>${secret(c.peasants)}</td>
                <td>${c.hidden ? hiddenMark : `${c.revolt_risk}/6`}</td>
            </tr>
        `;
    }).join('');

    const notes = ['♥ comes back once from defeat', '♡ already came back once'];
    if (anyHidden) notes.push('* army size as of the last war');
    countriesDisplay.innerHTML = `
        <table class="realm-table">
            <thead><tr>
                <th>Realm</th>
                <th title="Health points, out of ${MAX_HP}">HP</th>
                <th>Gold</th>
                <th>Army</th>
                <th title="Peasants">Peas.</th>
                <th title="Revolt risk, out of 6">Revolt</th>
            </tr></thead>
            <tbody>${rows}</tbody>
        </table>
        <div class="stat-footnote">${notes.join(' · ')}</div>
    `;
}

// One compact row per merchant, sorted by country, the player's own first
function renderMerchants(state) {
    const merchants = Object.values(state.merchants || {}).sort((a, b) =>
        (b.player_id === currentUser) - (a.player_id === currentUser) ||
        a.country_id.localeCompare(b.country_id) ||
        a.player_id.localeCompare(b.player_id));

    if (merchants.length === 0) {
        merchantsDisplay.innerHTML = '<div class="empty-note">No merchants yet</div>';
        return;
    }

    const hiddenMark = '<span class="secret">?</span>';
    const rows = merchants.map(m => {
        const isYou = m.player_id === currentUser;
        const where = m.arriving
            ? `<span title="On the way: joins at the start of the next round">→ ${esc(m.country_id)}</span>`
            : `in ${esc(m.country_id)}`;
        return `
            <tr class="${isYou ? 'is-you' : ''}">
                <td>
                    <div class="realm-name-cell">
                        ${emblem('coin')}
                        <div>
                            <div class="realm-name">${esc(m.player_id)}${isYou ? '<span class="you-tag">You</span>' : ''}</div>
                            <div class="realm-sub">${where}</div>
                        </div>
                    </div>
                </td>
                <td class="gold">${m.purse_hidden ? hiddenMark : m.stored_gold}</td>
                <td>${m.hidden ? hiddenMark : (m.hidden_gold || 0)}</td>
                <td>${m.hidden ? hiddenMark : m.invested_gold}</td>
            </tr>
        `;
    }).join('');

    merchantsDisplay.innerHTML = `
        <table class="realm-table">
            <thead><tr>
                <th>Merchant</th>
                <th title="Gold the monarch can tax">Purse</th>
                <th title="Hidden gold: safe from tax and secret from the monarch">Hidden</th>
                <th>Invested</th>
            </tr></thead>
            <tbody>${rows}</tbody>
        </table>
    `;
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
    updateAssignSelects();
}

// Fills the "Change Player Role" dropdowns: every connected player or player
// already in the game (with their current role), and every living country
function updateAssignSelects() {
    const playerSelect = document.getElementById('assign-player-id');
    const countrySelect = document.getElementById('assign-country-select');
    if (!playerSelect || !countrySelect) return;

    const roles = {};
    for (const country of Object.values((gameState && gameState.countries) || {})) {
        if (country.monarch_id && !country.is_republic) {
            roles[country.monarch_id] = `monarch of ${country.country_id}`;
        }
    }
    for (const merchant of Object.values((gameState && gameState.merchants) || {})) {
        roles[merchant.player_id] = `merchant in ${merchant.country_id}`;
    }
    const players = [...new Set([...connectedPlayers, ...Object.keys(roles)])].sort();

    const prevPlayer = playerSelect.value;
    playerSelect.innerHTML = '<option value="">Select Player...</option>';
    players.forEach(name => {
        const option = document.createElement('option');
        option.value = name;
        option.textContent = roles[name] ? `${name} (${roles[name]})` : `${name} (no role)`;
        playerSelect.appendChild(option);
    });
    if (prevPlayer) playerSelect.value = prevPlayer;

    const prevCountry = countrySelect.value;
    countrySelect.innerHTML = '';
    for (const [id, country] of Object.entries((gameState && gameState.countries) || {})) {
        if (country.hp <= 0) continue;
        const option = document.createElement('option');
        option.value = id;
        option.textContent = id;
        countrySelect.appendChild(option);
    }
    if (prevCountry) countrySelect.value = prevCountry;
}

// Shows the admin the numbers behind the most recent war phase
function renderWarReport(history) {
    const report = document.getElementById('war-report');
    if (!report) return;

    const wars = (history.state_snapshots || []).filter(s => s.phase === 'war');
    if (wars.length === 0) {
        report.innerHTML = '<div class="no-players">No war fought yet</div>';
        return;
    }
    const war = wars[wars.length - 1];
    const events = war.events || [];

    report.innerHTML = '';
    const title = document.createElement('div');
    title.className = 'war-report-title';
    title.textContent = `Round ${war.turn}`;
    report.appendChild(title);

    const battles = events.filter(e => e.type === 'battle_resolved');
    if (battles.length === 0) {
        const none = document.createElement('div');
        none.className = 'no-players';
        none.textContent = 'Nobody attacked';
        report.appendChild(none);
    } else {
        const rows = battles.map(e => {
            const d = e.data || {};
            const outcome = d.winner_id
                ? `${esc(d.winner_id)} wins, ${esc(d.winner_id === d.attacker_id ? d.defender_id : d.attacker_id)} takes ${d.damage} damage`
                : 'Draw, no damage';
            return `
                <tr>
                    <td><strong>${esc(d.attacker_id)}</strong> <span class="secret">(${d.attacker_strength})</span></td>
                    <td><strong>${esc(d.defender_id)}</strong> <span class="secret">(${d.defender_strength})</span></td>
                    <td>${outcome}</td>
                </tr>
            `;
        }).join('');
        report.insertAdjacentHTML('beforeend', `
            <table class="report-table">
                <thead><tr><th>Attacker</th><th>Defender</th><th>Outcome</th></tr></thead>
                <tbody>${rows}</tbody>
            </table>
        `);
    }

    // Everything else that came out of the war: republic votes, conquests,
    // deposed monarchs and army upkeep
    const shown = ['republic_war_vote', 'annexation', 'republic_fallen', 'monarch_deposed', 'army_maintenance'];
    const others = events.filter(e => shown.includes(e.type));
    if (others.length > 0) {
        const list = document.createElement('ul');
        list.className = 'war-events';
        others.forEach(e => {
            const item = document.createElement('li');
            item.textContent = e.message;
            list.appendChild(item);
        });
        report.appendChild(list);
    }
}

function plural(n, word) {
    return `${n} ${word}${n === 1 ? '' : 's'}`;
}

// Sums up a republic's war vote: how many merchants backed each target
function describeWarVote(d) {
    const votes = Object.entries(d.votes || {}).sort((a, b) => b[1] - a[1]);
    const tally = votes.length > 0
        ? votes.map(([target, n]) => `${target} ${n}`).join(', ')
        : 'nobody voted to attack';
    const result = d.target_id ? `attacks ${d.target_id}` : 'no majority, no attack';
    return `${d.country_id} war vote (${plural(d.merchants, 'merchant')}): ${tally}. Result: ${result}`;
}

// Turns a game event into a readable result line, or null for events that
// are not worth a line of their own (income, payouts, arrivals and so on).
// kind picks the colour: success, fail, vote, exile or info.
function describeResult(e) {
    const d = e.data || {};
    const rebels = (d.participants || []).length;
    switch (e.type) {
        case 'peasant_tax':
            if (d.revolted) {
                return { kind: 'fail', text: `${d.country_id}: high tax FAILED. The peasants revolted and paid nothing` };
            }
            return { kind: 'success', text: `${d.country_id}: ${d.high_tax ? 'high' : 'low'} tax succeeded, ${d.amount} gold collected` };
        case 'peasant_revolt':
            return { kind: 'fail', text: `${d.country_id} took ${d.damage} damage from the peasant revolt` };
        case 'republic_tax_vote':
            return { kind: 'vote', text: e.message };
        case 'republic_war_vote':
            return { kind: 'vote', text: describeWarVote(d) };
        case 'battle_resolved': {
            const outcome = d.winner_id
                ? `${d.winner_id} wins, ${d.winner_id === d.attacker_id ? d.defender_id : d.attacker_id} takes ${d.damage} damage`
                : 'draw, no damage';
            return { kind: 'info', text: `${d.attacker_id} (${d.attacker_strength}) attacks ${d.defender_id} (${d.defender_strength}): ${outcome}` };
        }
        case 'revolt_success':
            return { kind: 'success', text: `Revolt in ${d.country_id} SUCCEEDED: ${plural(rebels, 'rebel')} with ${d.total_gold} gold beat ${d.defense_gold} defending gold` };
        case 'revolt_failed':
            return { kind: 'fail', text: `Revolt in ${d.country_id} FAILED: ${plural(rebels, 'rebel')} with ${d.total_gold} gold did not beat ${d.defense_gold} defending gold, and lost ${d.gold_lost} gold to the monarch` };
        case 'monarch_deposed': {
            const how = { 'conquest': 'was conquered', 'revolution': 'was overthrown', 'peasant revolt': 'lost the throne to a peasant revolt' }[d.reason] || 'was deposed';
            const where = d.to_country
                ? `and went to ${d.to_country} as a merchant`
                : 'and left the game (no country left to go to)';
            return { kind: 'exile', text: `Monarch ${d.monarch_id} of ${d.from_country} ${how} ${where}` };
        }
        case 'exile_relocated':
            return { kind: 'exile', text: `${d.monarch_id} could not settle in ${d.intended}, which fell, and went to ${d.to_country} instead` };
        case 'annexation':
        case 'country_collapsed':
        case 'republic_fallen':
        case 'republic_abandoned':
        case 'treasury_split':
        case 'merchant_fled':
            return { kind: 'info', text: e.message };
        default:
            return null;
    }
}

function appendResults(container, events) {
    let shown = 0;
    (events || []).forEach(e => {
        const result = describeResult(e);
        if (!result) return;
        const line = document.createElement('div');
        line.className = `history-result result-${result.kind}`;
        line.textContent = result.text;
        container.appendChild(line);
        shown++;
    });
    return shown;
}

// Shows the admin, in place of the actions they do not have, what the most
// recently finished phase did
function renderLastResults(history) {
    const snapshots = history.state_snapshots || [];
    actionsList.innerHTML = '';
    if (snapshots.length === 0) {
        actionsList.innerHTML = '<div class="no-players">No phase finished yet</div>';
        return;
    }
    const last = snapshots[snapshots.length - 1];
    const title = document.createElement('div');
    title.className = 'war-report-title';
    title.textContent = `Round ${last.turn}, ${formatPhase(last.phase)}`;
    actionsList.appendChild(title);
    if (appendResults(actionsList, last.events) === 0) {
        const none = document.createElement('div');
        none.className = 'no-players';
        none.textContent = 'Nothing to report';
        actionsList.appendChild(none);
    }
}

// Shows the admin how the merchants voted: each republic's most recent tax
// and war vote, with who voted which way, and the most recent merchant revolts
function renderVoteReport(history) {
    const report = document.getElementById('vote-report');
    if (!report) return;

    const snapshots = history.state_snapshots || [];
    const actions = history.actions || [];
    const lastIndex = (phase, type) => {
        for (let i = snapshots.length - 1; i >= 0; i--) {
            if (snapshots[i].phase === phase && (snapshots[i].events || []).some(e => type.includes(e.type))) return i;
        }
        return -1;
    };

    // Which country each merchant belonged to when a phase began: the state
    // the phase before it ended in
    const countryOf = (i, merchantId) => {
        const state = (snapshots[i - 1] || snapshots[i]).state || {};
        const m = (state.merchants || {})[merchantId];
        return m ? m.country_id : '';
    };
    // Who voted which way in one republic during one phase
    const ballots = (i, countryId, describe) => {
        const snap = snapshots[i];
        const groups = {};
        actions.forEach(a => {
            if (a.turn !== snap.turn || a.phase !== snap.phase) return;
            const label = describe(a.action);
            if (!label || countryOf(i, a.action.merchant_id) !== countryId) return;
            (groups[label] = groups[label] || []).push(a.player_id);
        });
        return Object.entries(groups).map(([label, who]) => `${label}: ${who.join(', ')}`).join(' · ');
    };

    const sections = [];

    const tax = lastIndex('taxation', ['republic_tax_vote']);
    if (tax >= 0) {
        const lines = snapshots[tax].events.filter(e => e.type === 'republic_tax_vote').map(e => ({
            country: e.data.country_id,
            result: e.message,
            detail: ballots(tax, e.data.country_id, a => ({ vote_tax_high: 'High', vote_tax_low: 'Low' })[a.type]),
        }));
        sections.push({ title: `Tax vote, round ${snapshots[tax].turn}`, lines });
    }

    const war = lastIndex('war', ['republic_war_vote']);
    if (war >= 0) {
        const lines = snapshots[war].events.filter(e => e.type === 'republic_war_vote').map(e => ({
            country: e.data.country_id,
            result: describeWarVote(e.data || {}),
            detail: ballots(war, e.data.country_id, a =>
                a.type === 'vote_attack' ? `Attack ${a.target_id}` : a.type === 'vote_no_attack' ? 'No attack' : null),
        }));
        sections.push({ title: `War vote, round ${snapshots[war].turn}`, lines });
    }

    const revolt = lastIndex('assessment', ['revolt_success', 'revolt_failed']);
    if (revolt >= 0) {
        const lines = snapshots[revolt].events
            .filter(e => e.type === 'revolt_success' || e.type === 'revolt_failed')
            .map(e => ({ country: e.data.country_id, result: describeResult(e).text, detail: `Rebels: ${(e.data.participants || []).join(', ')}` }));
        sections.push({ title: `Revolts, round ${snapshots[revolt].turn}`, lines });
    }

    report.innerHTML = '';
    if (sections.length === 0) {
        report.innerHTML = '<div class="no-players">No votes yet</div>';
        return;
    }
    sections.forEach(section => {
        const title = document.createElement('div');
        title.className = 'war-report-title';
        title.textContent = section.title;
        report.appendChild(title);
        const rows = section.lines.map(line => `
            <tr>
                <td><strong>${esc(line.country)}</strong></td>
                <td>${esc(line.result)}${line.detail ? `<div class="secret">${esc(line.detail)}</div>` : ''}</td>
            </tr>
        `).join('');
        report.insertAdjacentHTML('beforeend', `
            <table class="report-table">
                <tbody>${rows}</tbody>
            </table>
        `);
    });
}

function renderConnectedPlayers() {
    const list = document.getElementById('connected-players-list');
    if (!list) return;

    list.innerHTML = '';
    const count = document.getElementById('ready-count');
    if (connectedPlayers.length === 0) {
        list.innerHTML = '<div class="no-players">No players connected</div>';
        count.textContent = '';
        return;
    }

    const readyCount = connectedPlayers.filter(name => readyPlayers.includes(name)).length;
    count.textContent = `${readyCount} of ${connectedPlayers.length} done`;
    count.className = readyCount === connectedPlayers.length ? 'all-ready' : '';

    connectedPlayers.forEach(name => {
        const isReady = readyPlayers.includes(name);
        const tag = document.createElement('span');
        tag.className = isReady ? 'player-tag ready' : 'player-tag';
        tag.textContent = (isReady ? '✓ ' : '') + name;
        tag.title = isReady ? 'Done with this phase' : 'Still choosing moves';
        list.appendChild(tag);
    });
}

// Shows a player whether they have told the admin they are done
function renderReadyButton() {
    if (currentUser === 'admin') return;
    const isReady = readyPlayers.includes(currentUser);
    const btn = document.getElementById('ready-btn');
    btn.textContent = isReady ? '✓ Ready (click to undo)' : "I'm done with this phase";
    btn.classList.toggle('ready', isReady);
    document.getElementById('ready-note').textContent = isReady
        ? 'The admin can see you are done. Changing a move will undo this.'
        : 'Let the admin know you have finished your moves.';
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
        empty.className = 'empty-note';
        empty.textContent = 'No moves available right now';
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
            btn.className = `action-btn ${actionTone(action.type)}`;
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
            btn.className = `action-btn ${actionTone(action.type)}`;
            btn.textContent = formatAction(action);
            btn.addEventListener('click', () => {
                send({ type: 'submit', action: action });
            });
            actionsList.appendChild(btn);
        }
    });
}

// Colours a move button the way the style kit colours its side: purple for
// the crown's moves, brass for merchant gold, oxblood for revolts and attacks
function actionTone(type) {
    if (['revolt', 'attack', 'vote_attack'].includes(type)) return 'tone-blood';
    if (['merchant_invest', 'merchant_hide', 'merchant_unhide', 'contribute_army', 'monarch_invest'].includes(type)) return 'tone-coin';
    if (['tax_peasants_low', 'tax_peasants_high', 'tax_merchants', 'build_army'].includes(type)) return 'tone-crown';
    return '';
}

function renderQueuedActions(actions) {
    queuedActionsList.innerHTML = '';

    if (!actions || actions.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'empty-note';
        empty.textContent = 'No queued moves';
        queuedActionsList.appendChild(empty);
        return;
    }

    actions.forEach(action => {
        const item = document.createElement('div');
        item.className = 'queued-action-item';

        const playerLabel = document.createElement('span');
        playerLabel.className = 'queued-player';
        playerLabel.textContent = action.player_id + ': ';

        const actionText = document.createElement('span');
        actionText.textContent = formatAction(action);

        item.appendChild(playerLabel);
        item.appendChild(actionText);
        queuedActionsList.appendChild(item);
    });

    // Show "Cancel All" button if the current user has queued actions
    const hasOwnActions = actions.some(a => a.player_id === currentUser);
    if (hasOwnActions && currentUser !== 'admin') {
        const cancelBtn = document.createElement('button');
        cancelBtn.className = 'cancel-btn';
        cancelBtn.textContent = 'Cancel All';
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

        const actionText = document.createElement('div');
        actionText.textContent = formatAction(rejected.action);

        const reasonText = document.createElement('div');
        reasonText.className = 'rejected-reason';
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
            return `Gift to ${action.merchant_id}`;
        case 'build_army':
            return 'Build Army';
        case 'contribute_army':
            return 'Contribute to Army';
        case 'merchant_hide':
            return 'Hide Gold';
        case 'merchant_unhide':
            return 'Unhide Gold';
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
            return `Gift ${action.amount} to ${action.merchant_id}`;
        case 'merchant_hide':
            return `Hide ${action.amount || 0} Gold`;
        case 'merchant_unhide':
            return `Unhide ${action.amount} Gold`;
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

    // Group actions by phase, along with what each finished phase did. Only
    // the admin (and everyone in an open game) gets those results.
    const actionsByPhase = {};
    const resultsByPhase = {};
    const actions = history.actions || [];
    const keyOf = entry => `Round ${entry.turn} · ${formatPhase(entry.phase)}`;

    (history.state_snapshots || []).forEach(snapshot => {
        actionsByPhase[keyOf(snapshot)] = actionsByPhase[keyOf(snapshot)] || [];
        resultsByPhase[keyOf(snapshot)] = snapshot.events || [];
    });
    actions.forEach(entry => {
        const key = keyOf(entry);
        if (!actionsByPhase[key]) {
            actionsByPhase[key] = [];
        }
        actionsByPhase[key].push(entry);
    });

    // Newest phase first. Only the newest starts open; the rest fold away
    // unless the viewer opened them.
    let newestShown = true;
    Object.keys(actionsByPhase).reverse().forEach(phaseKey => {
        const group = document.createElement('details');
        group.className = 'history-phase-group';

        const header = document.createElement('summary');
        header.className = 'history-phase-header';
        header.textContent = phaseKey;
        const moves = actionsByPhase[phaseKey].length;
        if (moves > 0) {
            const count = document.createElement('span');
            count.className = 'history-count';
            count.textContent = plural(moves, 'move');
            header.appendChild(count);
        }
        // Remember the viewer's choice, as the chronicle redraws every few seconds
        header.addEventListener('click', () => {
            historyOpen[phaseKey] = !group.open;
        });
        group.appendChild(header);

        const body = document.createElement('div');
        body.className = 'history-body';

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
                timeSpan.textContent = time.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
            }

            actionDiv.appendChild(playerSpan);
            actionDiv.appendChild(actionSpan);
            actionDiv.appendChild(timeSpan);
            body.appendChild(actionDiv);
        });

        if (resultsByPhase[phaseKey]) {
            const results = document.createElement('div');
            const label = document.createElement('div');
            label.className = 'history-results-label';
            label.textContent = 'Results';
            results.appendChild(label);
            if (appendResults(results, resultsByPhase[phaseKey]) > 0) {
                body.appendChild(results);
            }
        }

        // Skip phases where nobody did anything and nothing came of it
        if (body.children.length > 0) {
            group.appendChild(body);
            group.open = phaseKey in historyOpen ? historyOpen[phaseKey] : newestShown;
            newestShown = false;
            historyDisplay.appendChild(group);
        }
    });

    // Show message if no history
    if (historyDisplay.children.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'empty-note';
        empty.textContent = 'Nothing has happened yet';
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
