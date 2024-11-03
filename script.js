// Initialize WebSocket connection
const ws = new WebSocket("ws://localhost:8080/ws");

// Initialize chess game and board
let game = new Chess();
let board = null;
let moveSound = new Audio('move.mp3');
let playerColor = null;
let lastMove = null; // Track last move to prevent duplicates

// Board configuration
const config = {
    draggable: true,
    position: 'start',
    onDragStart: onDragStart,
    onDrop: onDrop,
    onSnapEnd: onSnapEnd
};

// Initialize the board
board = Chessboard('board', config);

function onDragStart(source, piece, position, orientation) {
    if (game.game_over() || 
        !playerColor || 
        (game.turn() === 'w' && playerColor !== 'white') ||
        (game.turn() === 'b' && playerColor !== 'black')) {
        return false;
    }

    if ((playerColor === 'white' && piece.search(/^b/) !== -1) ||
        (playerColor === 'black' && piece.search(/^w/) !== -1)) {
        return false;
    }

    return true;
}

function onDrop(source, target) {
    const move = game.move({
        from: source,
        to: target,
        promotion: 'q'
    });

    if (move === null) return 'snapback';

    lastMove = { from: source, to: target }; // Store last move
    
    console.log(game.fen())

    ws.send(JSON.stringify({
        type: 'move',
        from: source,
        to: target,
        promotion: 'q',
        fen: game.fen()
    }));

    updateStatus();
    playMoveSound();
}

function onSnapEnd() {
    board.position(game.fen());
}

function updateStatus() {
    let status = '';
    let moveColor = game.turn() === 'b' ? 'Black' : 'White';

    if (playerColor) {
        document.getElementById('player-color').textContent = `You are playing as ${playerColor}`;
    } else {
        document.getElementById('player-color').textContent = 'Spectating';
    }

    if (game.in_checkmate()) {
        status = `Game over, ${moveColor} is in checkmate.`;
    } else if (game.in_draw()) {
        status = 'Game over, drawn position';
    } else {
        status = `${moveColor}'s turn`;
        if (game.in_check()) {
            status += `, ${moveColor} is in check`;
        }
    }

    document.getElementById('turn-indicator').textContent = status;
}

function updateMoveHistory(move) {
    if (!move) return;
    
    const moveHistory = document.getElementById('move-history');
    const moveNumber = Math.ceil(game.history().length / 2);
    const moveText = `${moveNumber}. ${move.from}-${move.to}`;
    
    const moveElement = document.createElement('p');
    moveElement.textContent = moveText;
    moveHistory.appendChild(moveElement);
    moveHistory.scrollTop = moveHistory.scrollHeight;
}

function restartGame() {
    if (!playerColor) {
        alert("Only players can restart the game!");
        return;
    }
    game = new Chess();
    board.position('start');
    document.getElementById('move-history').innerHTML = '';
    document.getElementById('turn-indicator').textContent = "White's turn";
    ws.send(JSON.stringify({ type: 'restart' }));
}

function playMoveSound() {
    moveSound.currentTime = 0;
    moveSound.play().catch(e => console.log('Error playing sound:', e));
}

// Handle incoming WebSocket messages
ws.onmessage = function(event) {
    try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'color') {
            playerColor = msg.color;
            if (playerColor) {
                board.orientation(playerColor);
                document.getElementById('player-color').textContent = `You are playing as ${playerColor}`;
            } else {
                document.getElementById('player-color').textContent = 'Spectating (game is full)';
            }
        }
        else if (msg.type === 'gameState') {
            // Update game state for new connections
            game.load(msg.fen);
            board.position(msg.fen);
            
            document.getElementById('move-history').innerHTML = msg.moveHistory? msg.moveHistory : '';
            document.getElementById('messages').innerHTML = msg.chatHistory? msg.chatHistory : '';
            updateStatus();
        }
        else if (msg.type === 'move') {
            // Check if this is our own move (prevent duplicate)
            if (!lastMove || lastMove.from !== msg.from || lastMove.to !== msg.to) {
                game.move({
                    from: msg.from,
                    to: msg.to,
                    promotion: msg.promotion
                });
                board.position(game.fen());
                updateStatus();
                updateMoveHistory({ from: msg.from, to: msg.to });
                playMoveSound();
            }
            lastMove = null; // Reset last move
        } else if (msg.type === 'chat') {
            const messages = document.getElementById('messages');
            messages.innerHTML += `<p>${msg.message}</p>`;
            messages.scrollTop = messages.scrollHeight;
        } else if (msg.type === 'restart') {
            game = new Chess();
            board.position('start');
            document.getElementById('move-history').innerHTML = '';
            document.getElementById('turn-indicator').textContent = "White's turn";
        }
    } catch (error) {
        console.log("Error processing message:", error);
    }
};

function sendMessage() {
    const input = document.getElementById('messageInput');
    const message = input.value.trim();
    
    if (message) {
        ws.send(JSON.stringify({
            type: 'chat',
            message: message
        }));
        input.value = '';
    }
}

ws.onopen = function() {
    console.log('Connected to server');
};

ws.onclose = function() {
    document.getElementById('player-color').textContent = `Disconnected from server. Attempting to reconnect...`;
    console.log('Disconnected from server. Attempting to reconnect...');
    setTimeout(function() {
        ws = new WebSocket("ws://192.168.15.9:8080/ws");
    }, 1000);
};

window.addEventListener('dragover', function(e) {
    e.preventDefault();
});