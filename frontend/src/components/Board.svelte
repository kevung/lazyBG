<script>
    import { positionStore } from "../stores/positionStore";
    import { onMount, onDestroy } from "svelte";
    import Two from "two.js";
    import { get } from 'svelte/store';
    import { statusBarModeStore, isAnyModalOpenStore, showMetadataModalStore, showCandidateMovesStore, showMovesTableStore, showInitialPositionStore, candidatePreviewMoveStore } from '../stores/uiStore';
    import { selectedMoveStore, transcriptionStore } from '../stores/transcriptionStore';
    import { applyMove } from '../utils/positionCalculator.js';

    let mode;
    let showMetadataModal = false;
    let showCandidateMoves = false;
    let showMovesTable = false;
    let showInitialPosition = false;
    let currentMoveText = '';
    let cachedInitialPosition = null; // Cache initial position for edit mode final position display
    let two;
    let canvas;
    let width;
    let height;
    let unsubscribe;
    let cubePosition = { x: 0, y: 0 };
    
    let canvasCfg = {
        aspectFactor: 0.72,
    };

    let boardCfg = {
        widthFactor: 0.70, // Base width factor for board sizing - balanced space usage
        orientation: "right",
        fill: "#f0f0f0", // Light grey background
        stroke: "#333333", // Dark grey border
        linewidth: 3,
        triangle: {
            fill1: "#d9d9d9", // Light grey
            fill2: "#a6a6a6", // Slightly darker grey for balanced contrast
            stroke: "#333333",
            linewidth: 1.3, // Changed linewidth to 1
        },
        label: {
            size: 20,
            distanceToBoard: 0.15, // Reduced from 0.30 for tighter spacing
        },
        checker: {
            sizeFactor: 0.97,
            colors: ["#333333", "#ffffff"], // Dark grey and white checkers
            linewidth: 2.5 // Added linewidth property and set to 2
        }
    };
    
    statusBarModeStore.subscribe(value => {
        mode = value;
    });
    showMetadataModalStore.subscribe(value => {
        showMetadataModal = value;
    });
    showCandidateMovesStore.subscribe(value => {
        showCandidateMoves = value;
        // Trigger resize when panels change
        if (canvas && two) {
            setTimeout(() => resizeBoard(), 10);
        }
    });
    showMovesTableStore.subscribe(value => {
        showMovesTable = value;
        // Trigger resize when panels change
        if (canvas && two) {
            setTimeout(() => resizeBoard(), 10);
        }
    });
    
    showInitialPositionStore.subscribe(value => {
        showInitialPosition = value;
    });
    
    // Subscribe to candidate preview move (takes priority over transcription move)
    candidatePreviewMoveStore.subscribe(previewMove => {
        console.log('[Board] candidatePreviewMoveStore changed:', previewMove);
        if (previewMove) {
            // Use the preview move when cycling through candidates
            console.log('[Board] Setting currentMoveText to preview:', previewMove);
            currentMoveText = previewMove;
        } else {
            // Otherwise, get move from transcription
            console.log('[Board] No preview, getting move from transcription');
            const selectedMove = get(selectedMoveStore);
            const transcription = get(transcriptionStore);
            if (transcription && transcription.games && transcription.games.length > 0) {
                const { gameIndex, moveIndex, player } = selectedMove;
                const game = transcription.games[gameIndex];
                if (game && game.moves[moveIndex]) {
                    const move = game.moves[moveIndex];
                    const playerMove = player === 1 ? move.player1Move : move.player2Move;
                    currentMoveText = playerMove?.move || '';
                } else {
                    currentMoveText = '';
                }
            } else {
                currentMoveText = '';
            }
        }
    });
    
    // Subscribe to selected move to update when no preview is active
    selectedMoveStore.subscribe(selectedMove => {
        const previewMove = get(candidatePreviewMoveStore);
        if (!previewMove) {
            const transcription = get(transcriptionStore);
            if (transcription && transcription.games && transcription.games.length > 0) {
                const { gameIndex, moveIndex, player } = selectedMove;
                const game = transcription.games[gameIndex];
                if (game && game.moves[moveIndex]) {
                    const move = game.moves[moveIndex];
                    const playerMove = player === 1 ? move.player1Move : move.player2Move;
                    console.log('[Board] selectedMoveStore subscription - playerMove:', playerMove);
                    console.log('[Board] selectedMoveStore subscription - move from transcription:', playerMove?.move);
                    currentMoveText = playerMove?.move || '';
                } else {
                    console.log('[Board] selectedMoveStore subscription - no move found at index');
                    currentMoveText = '';
                }
            } else {
                console.log('[Board] selectedMoveStore subscription - no transcription');
                currentMoveText = '';
            }
        }
    });

    // Reactive statement: redraw board when currentMoveText changes (for candidate preview arrows)
    $: if (two && currentMoveText !== undefined) {
        console.log('[Board] currentMoveText changed, redrawing board:', currentMoveText);
        drawBoard();
    }

    function setBoardOrientation(orientation) {
        boardCfg.orientation = orientation;
        drawBoard();
    }

    function handleOrientationChange(event) {
        const isAnyModalOpen = get(isAnyModalOpenStore);
        if (isAnyModalOpen) return; // Disable orientation change when any modal is open
        if (event.ctrlKey && event.key === 'ArrowLeft') {
            setBoardOrientation("left");
        } else if (event.ctrlKey && event.key === 'ArrowRight') {
            setBoardOrientation("right");
        }
    }

    function resizeBoard() {
        const containerWidth = canvas.parentElement.clientWidth;
        const containerHeight = canvas.parentElement.clientHeight;
        
        // Use the constraining dimension to fit the board
        const widthBasedHeight = containerWidth * canvasCfg.aspectFactor;
        const heightBasedWidth = containerHeight / canvasCfg.aspectFactor;
        
        if (widthBasedHeight <= containerHeight) {
            // Width is the constraint
            width = containerWidth;
            height = widthBasedHeight;
        } else {
            // Height is the constraint
            width = heightBasedWidth;
            height = containerHeight;
        }
        
        two.width = width;
        two.height = height;
        two.renderer.setSize(width, height);
        drawBoard();
        two.update();
    }

    function logCanvasSize() {
        const actualWidth = canvas.clientWidth;
        const actualHeight = canvas.clientHeight;
        console.log("Actual canvas width: ", actualWidth, "Actual canvas height: ", actualHeight);
        console.log("Two.js width: ", two.width, "Two.js height: ", two.height);
    }

    onMount(() => {
        canvas = document.getElementById("backgammon-board");
        const params = { width: window.innerWidth, height: window.innerHeight };
        two = new Two(params).appendTo(canvas);

        // Calculate dimensions based on available space
        const containerWidth = canvas.parentElement.clientWidth;
        const containerHeight = canvas.parentElement.clientHeight;
        
        // Use the constraining dimension to fit the board
        const widthBasedHeight = containerWidth * canvasCfg.aspectFactor;
        const heightBasedWidth = containerHeight / canvasCfg.aspectFactor;
        
        if (widthBasedHeight <= containerHeight) {
            // Width is the constraint
            width = containerWidth;
            height = widthBasedHeight;
        } else {
            // Height is the constraint
            width = heightBasedWidth;
            height = containerHeight;
        }
        
        two.width = width;
        two.height = height;
        two.renderer.setSize(width, height);

        drawBoard();
        window.addEventListener("resize", resizeBoard);
        window.addEventListener("keydown", handleOrientationChange);

        // Add ResizeObserver to detect container size changes
        const resizeObserver = new ResizeObserver(() => {
            resizeBoard();
        });
        resizeObserver.observe(canvas.parentElement);

        unsubscribe = positionStore.subscribe(() => {
            const position = get(positionStore);
            const mode = get(statusBarModeStore);
            
            console.log('[Board] ======================================');
            console.log('[Board] positionStore.subscribe triggered');
            console.log('[Board] showInitialPosition:', showInitialPosition);
            console.log('[Board] mode:', mode);
            console.log('[Board] position.id:', position.id);
            console.log('[Board] position.player_on_roll:', position.player_on_roll);
            
            // Cache initial position ONLY when showInitialPosition is true
            // In Edit mode with final position display, positionStore contains the FINAL position,
            // so we should NOT cache it as "initial" - it will be used directly
            if (showInitialPosition) {
                console.log('[Board] Caching position as cachedInitialPosition (showing initial)');
                cachedInitialPosition = position;
            } else if (mode === 'EDIT') {
                console.log('[Board] In EDIT mode but final position - clearing cache to force direct use');
                cachedInitialPosition = null;
            } else {
                console.log('[Board] NOT caching position (showInitialPosition=false, mode!=EDIT)');
            }
            
            drawBoard();
            console.log("positionStore.subscribe - decision_type: ", position.decision_type); // Debug log
            console.log("positionStore: ", position);
        });

        logCanvasSize();
        window.addEventListener("resize", logCanvasSize);
    });

    onDestroy(() => {
        window.removeEventListener("resize", resizeBoard);
        window.removeEventListener("resize", logCanvasSize);
        window.removeEventListener("keydown", handleOrientationChange);
        if (unsubscribe) unsubscribe();
    });

    /**
     * Parse move text like "24/20 13/8" or "bar/21" and return array of moves
     * Each move is {from: number, to: number} where bar=25 for player on top, bar=0 for player on bottom
     * Supports repetition notation: "8/5(2)" means two moves from 8 to 5
     */
    function parseMoveText(moveText) {
        if (!moveText || moveText === 'Cannot Move' || moveText.includes('Doubles') || moveText.includes('Takes') || moveText.includes('Drops')) {
            return [];
        }
        
        const moves = [];
        const parts = moveText.split(/\s+/);
        
        for (const part of parts) {
            // Check for repetition notation: "8/5(2)" or "bar/23(3)"
            const repeatMatch = part.match(/^(.+?)\((\d+)\)$/);
            if (repeatMatch) {
                const baseMove = repeatMatch[1];
                const count = parseInt(repeatMatch[2], 10);
                const match = baseMove.match(/^(bar|\d+)\/(off|\d+)\*?$/i);
                if (match) {
                    let from = match[1].toLowerCase() === 'bar' ? 'bar' : parseInt(match[1]);
                    let to = match[2].toLowerCase() === 'off' ? 'off' : parseInt(match[2]);
                    // Add the move 'count' times
                    for (let i = 0; i < count; i++) {
                        moves.push({ from, to });
                    }
                }
            } else {
                const match = part.match(/^(bar|\d+)\/(off|\d+)\*?$/i);
                if (match) {
                    let from = match[1].toLowerCase() === 'bar' ? 'bar' : parseInt(match[1]);
                    let to = match[2].toLowerCase() === 'off' ? 'off' : parseInt(match[2]);
                    moves.push({ from, to });
                }
            }
        }
        
        return moves;
    }

    /**
     * Get the x, y coordinates for a point on the board
     */
    function getPointCoordinates(point, boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize) {
        let x, yBase;
        
        if (boardCfg.orientation === "right") {
            if (point === 0) { // Player 2's bar
                x = boardOrigXpos;
                yBase = boardOrigYpos + 0.5 * boardCheckerSize;
            } else if (point === 25) { // Player 1's bar
                x = boardOrigXpos;
                yBase = boardOrigYpos - 0.5 * boardCheckerSize;
            } else if (point === 'off-bottom') { // Bear off for bottom player
                x = boardOrigXpos + boardWidth / 2 + 1.2 * boardCheckerSize;
                yBase = boardOrigYpos + boardHeight / 2 - 3.7 * boardCheckerSize;
            } else if (point === 'off-top') { // Bear off for top player
                x = boardOrigXpos + boardWidth / 2 + 1.2 * boardCheckerSize;
                yBase = boardOrigYpos - boardHeight / 2 + 3.7 * boardCheckerSize;
            } else if (point <= 6) {
                x = boardOrigXpos + (7 - point) * boardCheckerSize;
                yBase = boardOrigYpos + 0.5 * boardHeight;
            } else if (point <= 12) {
                x = boardOrigXpos - (point - 6) * boardCheckerSize;
                yBase = boardOrigYpos + 0.5 * boardHeight;
            } else if (point <= 18) {
                x = boardOrigXpos - (19 - point) * boardCheckerSize;
                yBase = boardOrigYpos - 0.5 * boardHeight;
            } else {
                x = boardOrigXpos + (point - 18) * boardCheckerSize;
                yBase = boardOrigYpos - 0.5 * boardHeight;
            }
        } else if (boardCfg.orientation === "left") {
            if (point === 0) {
                x = boardOrigXpos;
                yBase = boardOrigYpos + 0.5 * boardCheckerSize;
            } else if (point === 25) {
                x = boardOrigXpos;
                yBase = boardOrigYpos - 0.5 * boardCheckerSize;
            } else if (point === 'off-bottom') {
                x = boardOrigXpos - boardWidth / 2 - 1.2 * boardCheckerSize;
                yBase = boardOrigYpos + boardHeight / 2 - 3.7 * boardCheckerSize;
            } else if (point === 'off-top') {
                x = boardOrigXpos - boardWidth / 2 - 1.2 * boardCheckerSize;
                yBase = boardOrigYpos - boardHeight / 2 + 3.7 * boardCheckerSize;
            } else if (point <= 6) {
                x = boardOrigXpos - (7 - point) * boardCheckerSize;
                yBase = boardOrigYpos + 0.5 * boardHeight;
            } else if (point <= 12) {
                x = boardOrigXpos + (point - 6) * boardCheckerSize;
                yBase = boardOrigYpos + 0.5 * boardHeight;
            } else if (point <= 18) {
                x = boardOrigXpos + (19 - point) * boardCheckerSize;
                yBase = boardOrigYpos - 0.5 * boardHeight;
            } else {
                x = boardOrigXpos - (point - 18) * boardCheckerSize;
                yBase = boardOrigYpos - 0.5 * boardHeight;
            }
        }
        
        return { x, y: yBase };
    }

    /**
     * Get the topmost checker position on a point (where a player would naturally pick it up)
     */
    function getCheckerPickupPosition(point, boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize) {
        const position = get(positionStore);
        const pointData = position.board.points[point];
        
        // Get base coordinates
        const baseCoords = getPointCoordinates(point, boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize);
        
        // Calculate the Y offset based on number of checkers
        let checkerCount = 0;
        if (pointData && pointData.checkers > 0) {
            checkerCount = Math.min(pointData.checkers, 5); // Max 5 visible
        }
        
        // Determine direction (up or down from base)
        let direction = 1; // default down
        if (point !== 0 && point <= 12 || point === 25) {
            direction = -1; // up
        }
        
        // Calculate Y position of topmost checker
        const yOffset = (checkerCount - 0.5) * boardCfg.checker.sizeFactor * boardCheckerSize * direction;
        
        return {
            x: baseCoords.x,
            y: baseCoords.y + yOffset
        };
    }

    /**
     * Draw arrows showing checker movements
     */
    function drawMoveArrows(boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize) {
        // Draw arrows only when showInitialPosition is true
        // When false, we show final position with checkers in their final locations
        if (!showInitialPosition || !currentMoveText) {
            return;
        }
        
        const moves = parseMoveText(currentMoveText);
        const position = get(positionStore);
        const playerOnRoll = position.player_on_roll;
        
        // Track how many checkers have been moved from each point to draw arrows from correct positions
        const checkersMovedFrom = {}; // point -> count of checkers moved from that point
        const checkersMovedTo = {}; // point -> count of checkers moved to that point
        
        for (const move of moves) {
            let fromPoint = move.from;
            let toPoint = move.to;
            
            // Convert 'bar' to appropriate bar point
            if (fromPoint === 'bar') {
                fromPoint = playerOnRoll === 0 ? 25 : 0;
            } else if (typeof fromPoint === 'number' && playerOnRoll === 1) {
                // Player 2 (white): convert from their perspective (1-24) to board coordinates
                // Player 2's point 1 = board point 24, Player 2's point 24 = board point 1
                fromPoint = 25 - fromPoint;
            }
            
            // Convert 'off' to appropriate bearoff position
            if (toPoint === 'off') {
                toPoint = playerOnRoll === 0 ? 'off-bottom' : 'off-top';
            } else if (typeof toPoint === 'number' && playerOnRoll === 1) {
                // Player 2 (white): convert from their perspective to board coordinates
                toPoint = 25 - toPoint;
            }
            
            // Track how many checkers have been moved from this point
            const movedFromCount = checkersMovedFrom[fromPoint] || 0;
            const movedToCount = checkersMovedTo[toPoint] || 0;
            
            // Get coordinates - from topmost checker on source point or from bar
            let fromCoords;
            if (typeof fromPoint === 'number' && fromPoint >= 0 && fromPoint <= 25) {
                const sourcePointData = position.board.points[fromPoint];
                const baseCoords = getPointCoordinates(fromPoint, boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize);
                
                // Calculate which checker we're moving (accounting for previously moved checkers)
                // Initial checker count from the position
                let initialCheckerCount = 0;
                if (sourcePointData && sourcePointData.checkers > 0) {
                    initialCheckerCount = Math.min(sourcePointData.checkers, 5); // Max 5 visible
                }
                
                // Add checkers that were moved TO this point in previous moves
                const checkersAddedHere = checkersMovedTo[fromPoint] || 0;
                const totalCheckers = initialCheckerCount + checkersAddedHere;
                
                // The checker to move is counting from the top down
                const checkerIndex = totalCheckers - movedFromCount - 1;
                
                // Ensure we don't get negative index
                const safeCheckerIndex = Math.max(0, checkerIndex);
                
                // Determine direction (up or down from base)
                let direction = 1; // default down
                if (fromPoint !== 0 && fromPoint <= 12 || fromPoint === 25) {
                    direction = -1; // up
                }
                
                // Calculate Y position of the checker we're moving
                const yOffset = (safeCheckerIndex + 0.5) * boardCfg.checker.sizeFactor * boardCheckerSize * direction;
                fromCoords = {
                    x: baseCoords.x,
                    y: baseCoords.y + yOffset
                };
            } else {
                fromCoords = getPointCoordinates(fromPoint, boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize);
            }
            
            // Get coordinates for destination - calculate where checker will land
            let toCoords;
            if (typeof toPoint === 'number' && toPoint >= 0 && toPoint <= 25) {
                const destPointData = position.board.points[toPoint];
                const baseCoords = getPointCoordinates(toPoint, boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize);
                
                // Calculate how many checkers are already there plus how many we've already moved there
                let existingCheckers = 0;
                if (destPointData && destPointData.checkers > 0) {
                    existingCheckers = Math.min(destPointData.checkers, 5);
                }
                
                // If there's an opponent blot (exactly 1 opponent checker), arrow should point to it
                // Player 1 checkers are positive, Player 2 checkers are negative
                const isHit = (playerOnRoll === 0 && existingCheckers === 1 && destPointData.checkers < 0) ||
                              (playerOnRoll === 1 && existingCheckers === 1 && destPointData.checkers > 0);
                
                let targetCheckerCount;
                if (isHit) {
                    // Arrow points to the opponent checker that will be hit (at position 0)
                    targetCheckerCount = 0;
                } else {
                    // Arrow points to where the checker will land (on top of existing checkers + previously moved)
                    targetCheckerCount = existingCheckers + movedToCount;
                }
                
                // The new checker will land on top of existing checkers
                let direction = 1;
                if (toPoint !== 0 && toPoint <= 12 || toPoint === 25) {
                    direction = -1;
                }
                
                const yOffset = (targetCheckerCount + 0.5) * boardCfg.checker.sizeFactor * boardCheckerSize * direction;
                toCoords = {
                    x: baseCoords.x,
                    y: baseCoords.y + yOffset
                };
            } else {
                toCoords = getPointCoordinates(toPoint, boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize);
            }
            
            // Update counters for next iteration
            checkersMovedFrom[fromPoint] = movedFromCount + 1;
            if (typeof toPoint === 'number') {
                checkersMovedTo[toPoint] = movedToCount + 1;
            }
            
            // Use red arrows with transparency
            const arrowColor = 'rgba(255, 50, 50, 0.7)';
            const arrowWidth = boardCheckerSize * 0.2;
            
            // Calculate arrow geometry
            const angle = Math.atan2(toCoords.y - fromCoords.y, toCoords.x - fromCoords.x);
            const arrowHeadSize = boardCheckerSize * 0.5;
            const arrowAngle = Math.PI / 5; // Slightly narrower arrow
            
            // Shorten the line so it stops before the arrow tip
            const lineEndX = toCoords.x - arrowHeadSize * 0.6 * Math.cos(angle);
            const lineEndY = toCoords.y - arrowHeadSize * 0.6 * Math.sin(angle);
            
            // Create arrow line that stops before the tip
            const line = two.makeLine(fromCoords.x, fromCoords.y, lineEndX, lineEndY);
            line.stroke = arrowColor;
            line.linewidth = arrowWidth;
            line.cap = 'round';
            
            // Create arrowhead
            const arrowPoint1 = {
                x: toCoords.x - arrowHeadSize * Math.cos(angle - arrowAngle),
                y: toCoords.y - arrowHeadSize * Math.sin(angle - arrowAngle)
            };
            const arrowPoint2 = {
                x: toCoords.x - arrowHeadSize * Math.cos(angle + arrowAngle),
                y: toCoords.y - arrowHeadSize * Math.sin(angle + arrowAngle)
            };
            
            const arrowHead = two.makePath(
                toCoords.x, toCoords.y,
                arrowPoint1.x, arrowPoint1.y,
                arrowPoint2.x, arrowPoint2.y,
                toCoords.x, toCoords.y
            );
            arrowHead.fill = arrowColor;
            arrowHead.stroke = arrowColor;
            arrowHead.linewidth = 0;
        }
    }

    function drawBoard() {
        two.clear();

        const boardAspectFactor = 11 / 13;
        const boardWidth = boardCfg.widthFactor * width;
        const boardHeight = boardAspectFactor * boardWidth;
        const boardCheckerSize = boardHeight / 11;
        const boardTriangleHeight = 5 * boardCheckerSize;
        const boardOrigXpos = width / 2;
        const boardOrigYpos = height / 2;
        console.log("width: ", width, "height: ", height);
        console.log("boardOrigXpos: ", boardOrigXpos, "boardOrigYpos: ", boardOrigYpos);
        console.log("two.width: ", two.width, "two.height: ", two.height);

        const position = get(positionStore);
        console.log("drawBoard - decision_type: ", position.decision_type); // Debug log

        function createTriangle(x, y, flip) {
            if (flip == false) {
                const triangle = two.makePath(
                    x,
                    y,
                    x + boardCheckerSize,
                    y,
                    x + 0.5 * boardCheckerSize,
                    y + 5 * boardCheckerSize,
                );
                triangle.stroke = boardCfg.triangle.stroke;
                triangle.linewidth = boardCfg.triangle.linewidth;
                return triangle;
            } else {
                const triangle = two.makePath(
                    x,
                    y + boardTriangleHeight,
                    x + boardCheckerSize,
                    y + boardTriangleHeight,
                    x + 0.5 * boardCheckerSize,
                    y + boardTriangleHeight - 5 * boardCheckerSize,
                );

                triangle.stroke = boardCfg.triangle.stroke;
                triangle.linewidth = boardCfg.triangle.linewidth;
                return triangle;
            }
        }

        function createQuadrant(x, y, flip) {
            let quadrant = two.makeGroup();
            for (let i = 0; i < 6; i++) {
                const offsetX = x + i * boardCheckerSize;
                const offsetY = y;
                const t = createTriangle(offsetX, offsetY, flip);
                if (i % 2 == 1) {
                    t.fill = boardCfg.triangle.fill1;
                } else {
                    t.fill = boardCfg.triangle.fill2;
                }

                //invert color
                if (flip) {
                    if (i % 2 == 1) {
                        t.fill = boardCfg.triangle.fill2;
                    } else {
                        t.fill = boardCfg.triangle.fill1;
                    }
                }

                quadrant.add(t);
            }
            return quadrant;
        }

        function createLabels() {
            let labels = two.makeGroup();
            const position = get(positionStore);
            const flip = position.player_on_roll === 1;

            if (boardCfg.orientation === "right") {
                for (let i = 0; i < 6; i++) {
                    const x = boardOrigXpos + (6 - i) * boardCheckerSize;
                    const y = boardOrigYpos + 0.5 * boardHeight + (boardCfg.label.distanceToBoard + 0.05) * boardCheckerSize;
                    const t = two.makeText((flip ? 24 - i : i + 1).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "top";
                    labels.add(t);
                }
                for (let i = 6; i < 12; i++) {
                    const x = boardOrigXpos - (i - 5) * boardCheckerSize;
                    const y = boardOrigYpos + 0.5 * boardHeight + (boardCfg.label.distanceToBoard + 0.05) * boardCheckerSize;
                    const t = two.makeText((flip ? 24 - i : i + 1).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "top";
                    labels.add(t);
                }
                for (let i = 12; i < 18; i++) {
                    const x = boardOrigXpos + (i - 18) * boardCheckerSize;
                    const y = boardOrigYpos - 0.5 * boardHeight - (boardCfg.label.distanceToBoard + 0.02) * boardCheckerSize;
                    const t = two.makeText((flip ? 24 - i : i + 1).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "bottom";
                    labels.add(t);
                }
                for (let i = 18; i < 24; i++) {
                    const x = boardOrigXpos + (i - 17) * boardCheckerSize;
                    const y = boardOrigYpos - 0.5 * boardHeight - (boardCfg.label.distanceToBoard + 0.02) * boardCheckerSize;
                    const t = two.makeText((flip ? 24 - i : i + 1).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "bottom";
                    labels.add(t);
                }
            } else if (boardCfg.orientation === "left") {
                for (let i = 0; i < 6; i++) {
                    const x = boardOrigXpos - (6 - i) * boardCheckerSize;
                    const y = boardOrigYpos - 0.5 * boardHeight - (boardCfg.label.distanceToBoard + 0.02) * boardCheckerSize;
                    const t = two.makeText((flip ? i + 1 : 24 - i).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "bottom";
                    labels.add(t);
                }
                for (let i = 6; i < 12; i++) {
                    const x = boardOrigXpos + (i - 5) * boardCheckerSize;
                    const y = boardOrigYpos - 0.5 * boardHeight - (boardCfg.label.distanceToBoard + 0.02) * boardCheckerSize;
                    const t = two.makeText((flip ? i + 1 : 24 - i).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "bottom";
                    labels.add(t);
                }
                for (let i = 12; i < 18; i++) {
                    const x = boardOrigXpos - (i - 18) * boardCheckerSize;
                    const y = boardOrigYpos + 0.5 * boardHeight + (boardCfg.label.distanceToBoard + 0.05) * boardCheckerSize;
                    const t = two.makeText((flip ? i + 1 : 24 - i).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "top";
                    labels.add(t);
                }
                for (let i = 18; i < 24; i++) {
                    const x = boardOrigXpos - (i - 17) * boardCheckerSize;
                    const y = boardOrigYpos + 0.5 * boardHeight + (boardCfg.label.distanceToBoard + 0.05) * boardCheckerSize;
                    const t = two.makeText((flip ? i + 1 : 24 - i).toString(), x, y);
                    t.size = boardCfg.label.size;
                    t.alignment = "center";
                    t.baseline = "top";
                    labels.add(t);
                }
            }
            return labels;
        }

        /**
         * Convert positionStore format to positionCalculator format
         * positionStore: {board: {points: [{checkers: 5, color: 0}, ...]}}
         * positionCalculator: {points: [5, -3, 0, ...]} (positive = player 0, negative = player 1)
         */
        function convertToCalculatorFormat(storePosition) {
            const points = new Array(25).fill(0);
            storePosition.board.points.forEach((point, index) => {
                if (point && point.checkers > 0) {
                    // color 0 = positive numbers, color 1 = negative numbers
                    points[index] = point.color === 0 ? point.checkers : -point.checkers;
                }
            });
            return {
                points,
                bar: 0,
                off: 0,
                opponentBar: 0,
                opponentOff: 0
            };
        }
        
        /**
         * Convert positionCalculator format back to positionStore format
         */
        function convertFromCalculatorFormat(calcPosition, originalPosition) {
            const newPoints = calcPosition.points.map((value, index) => {
                if (value === 0) {
                    return { checkers: 0, color: -1 };
                } else if (value > 0) {
                    return { checkers: value, color: 0 };
                } else {
                    return { checkers: Math.abs(value), color: 1 };
                }
            });
            
            return {
                ...originalPosition,
                board: {
                    ...originalPosition.board,
                    points: newPoints
                }
            };
        }
        
        function drawCheckers() {
            let position = get(positionStore);
            const isEditMode = mode === 'EDIT';
            
            console.log('[Board] ========== drawCheckers ==========');
            console.log('[Board] isEditMode:', isEditMode);
            console.log('[Board] showInitialPosition:', showInitialPosition);
            console.log('[Board] currentMoveText:', currentMoveText);
            console.log('[Board] cachedInitialPosition exists:', !!cachedInitialPosition);
            console.log('[Board] positionStore.id:', position.id);
            console.log('[Board] positionStore.player_on_roll:', position.player_on_roll);
            
            // In EDIT mode with final position display:
            // Use cached initial position and apply the current edit to show correct final position
            if (isEditMode && !showInitialPosition && cachedInitialPosition && currentMoveText && currentMoveText.trim() !== '') {
                console.log('[Board] 🎯 EDIT mode with final position display - applying move manually');
                console.log('[Board] cachedInitialPosition.id:', cachedInitialPosition.id);
                console.log('[Board] cachedInitialPosition.player_on_roll:', cachedInitialPosition.player_on_roll);
                try {
                    // Use the cached initial position
                    const calcPosition = convertToCalculatorFormat(cachedInitialPosition);
                    console.log('[Board] Converted cachedInitialPosition to calculator format');
                    
                    // Apply the current edit to get final position
                    console.log('[Board] Applying move:', currentMoveText);
                    const result = applyMove(calcPosition, currentMoveText, cachedInitialPosition.player_on_roll === 0);
                    console.log('[Board] Move applied successfully');
                    
                    // Convert back to store format
                    position = convertFromCalculatorFormat(result.position, cachedInitialPosition);
                    console.log('[Board] ✓ Using manually calculated position');
                } catch (error) {
                    console.warn('[Board] Failed to apply move for final position in edit mode:', error);
                    console.log('[Board] ✗ Falling back to positionStore position');
                    // Fall back to showing position as-is
                }
            } else {
                console.log('[Board] Using positionStore as-is (not applying move manually)');
            }
            
            position.board.points.forEach((point, index) => {
                let x, yBase;
                if (boardCfg.orientation === "right") {
                    if (index === 0) {
                        x = boardOrigXpos;
                        yBase = boardOrigYpos + 0.5 * boardCheckerSize;
                    } else if (index === 25) {
                        x = boardOrigXpos;
                        yBase = boardOrigYpos - 0.5 * boardCheckerSize;
                    } else if (index <= 6) {
                        x = boardOrigXpos + (7 - index) * boardCheckerSize;
                        yBase = boardOrigYpos + 0.5 * boardHeight;
                    } else if (index <= 12) {
                        x = boardOrigXpos - (index - 6) * boardCheckerSize;
                        yBase = boardOrigYpos + 0.5 * boardHeight;
                    } else if (index <= 18) {
                        x = boardOrigXpos - (19 - index) * boardCheckerSize;
                        yBase = boardOrigYpos - 0.5 * boardHeight;
                    } else {
                        x = boardOrigXpos + (index - 18) * boardCheckerSize;
                        yBase = boardOrigYpos - 0.5 * boardHeight;
                    }
                } else if (boardCfg.orientation === "left") {
                    if (index === 0) {
                        x = boardOrigXpos;
                        yBase = boardOrigYpos + 0.5 * boardCheckerSize;
                    } else if (index === 25) {
                        x = boardOrigXpos;
                        yBase = boardOrigYpos - 0.5 * boardCheckerSize;
                    } else if (index <= 6) {
                        x = boardOrigXpos - (7 - index) * boardCheckerSize;
                        yBase = boardOrigYpos + 0.5 * boardHeight;
                    } else if (index <= 12) {
                        x = boardOrigXpos + (index - 6) * boardCheckerSize;
                        yBase = boardOrigYpos + 0.5 * boardHeight;
                    } else if (index <= 18) {
                        x = boardOrigXpos + (19 - index) * boardCheckerSize;
                        yBase = boardOrigYpos - 0.5 * boardHeight;
                    } else {
                        x = boardOrigXpos - (index - 18) * boardCheckerSize;
                        yBase = boardOrigYpos - 0.5 * boardHeight;
                    }
                }
                const checkersToDraw = Math.min(point.checkers, 5);
                for (let i = 0; i < checkersToDraw; i++) {
                    const y = yBase + (index !== 0 && index <= 12 || index === 25 ? -1 : 1) * (i + 0.5) * boardCfg.checker.sizeFactor * boardCheckerSize;
                    const checker = two.makeCircle(x, y, boardCfg.checker.sizeFactor * boardCheckerSize / 2);
                    checker.fill = boardCfg.checker.colors[point.color];
                    checker.stroke = boardCfg.triangle.stroke;
                    checker.linewidth = boardCfg.checker.linewidth; // Use checker linewidth
                    if (i === 4 && point.checkers > 5) {
                        const text = two.makeText(point.checkers.toString(), x, y);
                        text.size = 20; // Ensure consistent text size
                        text.alignment = "center";
                        text.baseline = "middle";
                        text.weight = "bold"; // Ensure consistent text weight
                        if (point.color === 0) {
                            text.fill = "#ffffff"; // Contrast color for black checker
                        } else if (point.color === 1) {
                            text.fill = "#333333"; // Contrast color for white checker
                        }
                    }
                }
            });

            // Draw checkers on the bar above the bar
            position.board.points.forEach((point, index) => {
                if (index === 0 || index === 25) {
                    let x = boardOrigXpos;
                    let yBase = index === 0 ? boardOrigYpos + 0.5 * boardCheckerSize : boardOrigYpos - 0.5 * boardCheckerSize;
                    const checkersToDraw = Math.min(point.checkers, 5);
                    for (let i = 0; i < checkersToDraw; i++) {
                        const y = yBase + (index === 0 ? 1 : -1) * (i + 0.5) * boardCfg.checker.sizeFactor * boardCheckerSize;
                        const checker = two.makeCircle(x, y, boardCfg.checker.sizeFactor * boardCheckerSize / 2);
                        checker.fill = boardCfg.checker.colors[point.color];
                        checker.stroke = boardCfg.triangle.stroke;
                        checker.linewidth = boardCfg.checker.linewidth; // Use checker linewidth
                        if (i === 4 && point.checkers > 5) {
                            const text = two.makeText(point.checkers.toString(), x, y);
                            text.size = 20; // Ensure consistent text size
                            text.alignment = "center";
                            text.baseline = "middle";
                            text.weight = "bold"; // Ensure consistent text weight
                            if (point.color === 0) {
                                text.fill = "#ffffff"; // Contrast color for black checker
                            } else if (point.color === 1) {
                                text.fill = "#333333"; // Contrast color for white checker
                            }
                        }
                    }
                }
            });
        }

        function drawDoublingCube() {
            const boardCheckerSize = (11 / 13) * (boardCfg.widthFactor * width) / 11;
            const boardOrigXpos = width / 2;
            const boardOrigYpos = height / 2;
            const boardWidth = boardCfg.widthFactor * width;

            // Get the value for the doubling cube
            const position = get(positionStore);
            const cubeValue = position.cube.value;
            const doublingCubeTextValue = Math.pow(2, cubeValue);

            // Determine the position of the doubling cube based on its owner
            const doublingCubeSize = 0.9 * boardCheckerSize; // Reduce the size of the doubling cube
            const gap = 0.75 * boardCheckerSize;

            if (position.cube.owner === -1) {
                cubePosition.x = boardOrigXpos - boardWidth / 2 - doublingCubeSize / 2 - gap;
                cubePosition.y = boardOrigYpos;
            } else if (position.cube.owner === 0) {
                cubePosition.x = boardOrigXpos - boardWidth / 2 - doublingCubeSize / 2 - gap;
                cubePosition.y = boardOrigYpos + 0.5 * boardHeight - 1.5 * boardCheckerSize;
            } else if (position.cube.owner === 1) {
                cubePosition.x = boardOrigXpos - boardWidth / 2 - doublingCubeSize / 2 - gap;
                cubePosition.y = boardOrigYpos - 0.5 * boardHeight + 1.5 * boardCheckerSize;
            }
            cubePosition.size = doublingCubeSize;

            const doublingCube = two.makeRectangle(
                cubePosition.x,
                cubePosition.y,
                doublingCubeSize,
                doublingCubeSize,
            );
            doublingCube.fill = "#ffffff"; // White doubling cube
            doublingCube.stroke = "#333333"; // Dark grey border
            doublingCube.linewidth = 2.5; // Adjust linewidth accordingly
            const doublingCubeText = two.makeText(doublingCubeTextValue.toString(), cubePosition.x, cubePosition.y);
            doublingCubeText.size = 34; // Checker size
            doublingCubeText.alignment = "center";
            doublingCubeText.baseline = "middle";
            doublingCubeText.translation.set(cubePosition.x, cubePosition.y + 0.05 * doublingCubeSize); // Center the text
        }

        function computePipCount() {
            const position = get(positionStore);
            let pipCount1 = 0;
            let pipCount2 = 0;

            position.board.points.forEach((point, index) => {
                if (point.color === 0) {
                    pipCount1 += point.checkers * index;
                } else if (point.color === 1) {
                    pipCount2 += point.checkers * (25 - index);
                }
            });

            return { pipCount1, pipCount2 };
        }

        function drawPipCounts() {
            const { pipCount1, pipCount2 } = computePipCount();

            const boardOrigXpos = width / 2;
            const boardOrigYpos = height / 2;
            const boardWidth = boardCfg.widthFactor * width;
            const boardCheckerSize = (11 / 13) * (boardCfg.widthFactor * width) / 11;

            const pipCountText1 = `pip: ${pipCount1}`;
            const pipCountText2 = `pip: ${pipCount2}`;

            const pipCount1Xpos = boardOrigXpos - boardWidth / 2 - 1.2 * boardCheckerSize;
            const pipCount1Ypos = boardOrigYpos + boardHeight / 2 + 0.2 * boardCheckerSize; // Align with score height

            const pipCount2Xpos = boardOrigXpos - boardWidth / 2 - 1.2 * boardCheckerSize;
            const pipCount2Ypos = boardOrigYpos - boardHeight / 2 - 0.2 * boardCheckerSize; // Align with score height

            const pipCountText1Element = two.makeText(pipCountText1, pipCount1Xpos, pipCount1Ypos);
            pipCountText1Element.size = 20; // Same size as score
            pipCountText1Element.alignment = "center";
            pipCountText1Element.baseline = "middle";
            pipCountText1Element.weight = "bold";

            const pipCountText2Element = two.makeText(pipCountText2, pipCount2Xpos, pipCount2Ypos);
            pipCountText2Element.size = 20; // Same size as score
            pipCountText2Element.alignment = "center";
            pipCountText2Element.baseline = "middle";
            pipCountText2Element.weight = "bold";
        }

        function drawBearoff() {
            const bearoff1 = get(positionStore).board.bearoff[0];
            const bearoff2 = get(positionStore).board.bearoff[1];
            const boardOrigXpos = width / 2;
            const boardOrigYpos = height / 2;
            const boardWidth = boardCfg.widthFactor * width;
            const boardCheckerSize = (11 / 13) * (boardCfg.widthFactor * width) / 11;
            const gap = 1.2 * boardCheckerSize;

            const bearoffText1 = `(${bearoff1} OFF)`;
            const bearoffText2 = `(${bearoff2} OFF)`;

            let bearoff1Xpos, bearoff1Ypos, bearoff2Xpos, bearoff2Ypos;

            if (boardCfg.orientation === "right") {
                bearoff1Xpos = boardOrigXpos + boardWidth / 2 + gap;
                bearoff1Ypos = boardOrigYpos + boardHeight / 2 - 3.7 * boardCheckerSize;

                bearoff2Xpos = boardOrigXpos + boardWidth / 2 + gap;
                bearoff2Ypos = boardOrigYpos - boardHeight / 2 + 3.7 * boardCheckerSize;
            } else if (boardCfg.orientation === "left") {
                bearoff1Xpos = boardOrigXpos - boardWidth / 2 - gap;
                bearoff1Ypos = boardOrigYpos + boardHeight / 2 - 3.7 * boardCheckerSize;

                bearoff2Xpos = boardOrigXpos - boardWidth / 2 - gap;
                bearoff2Ypos = boardOrigYpos - boardHeight / 2 + 3.7 * boardCheckerSize;
            }

            const bearoffText1Element = two.makeText(bearoffText1, bearoff1Xpos, bearoff1Ypos);
            bearoffText1Element.size = 20;
            bearoffText1Element.alignment = "center";
            bearoffText1Element.baseline = "middle";

            const bearoffText2Element = two.makeText(bearoffText2, bearoff2Xpos, bearoff2Ypos);
            bearoffText2Element.size = 20;
            bearoffText2Element.alignment = "center";
            bearoffText2Element.baseline = "middle";

            // Define score positions
            const score1Ypos = boardOrigYpos + boardHeight / 2 + 0.2 * boardCheckerSize;
            const score2Ypos = boardOrigYpos - boardHeight / 2 - 0.2 * boardCheckerSize;

            // Add transparent rectangles with red borders
            const rectangle1 = two.makeRectangle(bearoff1Xpos, (bearoff1Ypos + score1Ypos) / 2, 1.5 * boardCheckerSize, Math.abs(bearoff1Ypos - score1Ypos));
            rectangle1.fill = "transparent";
            rectangle1.stroke = "red"; // Make border visible
            rectangle1.linewidth = 0;

            const rectangle2 = two.makeRectangle(bearoff2Xpos, (bearoff2Ypos + score2Ypos) / 2, 1.5 * boardCheckerSize, Math.abs(bearoff2Ypos - score2Ypos));
            rectangle2.fill = "transparent";
            rectangle2.stroke = "red"; // Make border visible
            rectangle2.linewidth = 0;
        }

        function drawDice() {
            const position = get(positionStore);
            const playerOnRoll = position.player_on_roll;
            const dice = position.dice;
            const decisionType = position.decision_type;

            const boardOrigXpos = width / 2;
            const boardOrigYpos = height / 2;
            const boardWidth = boardCfg.widthFactor * width;
            const boardCheckerSize = (11 / 13) * (boardCfg.widthFactor * width) / 11;
            const gap = 0.325 * boardCheckerSize; // Move the dice closer to the board
            const diceSize = 0.7 * boardCheckerSize; // Reduce the size of the dice

            const diceXpos = boardOrigXpos + boardWidth / 2 + 2 * gap;
            const diceYpos = playerOnRoll === 0 ? boardOrigYpos + 0.5 * boardHeight - 1.5 * boardCheckerSize : boardOrigYpos - 0.5 * boardHeight + 1.5 * boardCheckerSize;

            dice.forEach((die, index) => {
                const dieXpos = diceXpos + index * (diceSize + gap);
                const dieElement = two.makeRectangle(dieXpos, diceYpos, diceSize, diceSize);
                dieElement.fill = "#ffffff"; // White dice
                dieElement.stroke = "#333333"; // Dark grey border
                dieElement.linewidth = 2.5; // Adjust linewidth accordingly

                if (decisionType === 0) {
                    // Draw dots for traditional dice
                    const dotPositions = [
                        [],
                        [[0, 0]],
                        [[-0.7, -0.7], [0.7, 0.7]],
                        [[-0.7, -0.7], [0, 0], [0.7, 0.7]],
                        [[-0.7, -0.7], [0.7, -0.7], [-0.7, 0.7], [0.7, 0.7]],
                        [[-0.7, -0.7], [0.7, -0.7], [0, 0], [-0.7, 0.7], [0.7, 0.7]],
                        [[-0.7, -0.7], [0.7, -0.7], [-0.7, 0], [0.7, 0], [-0.7, 0.7], [0.7, 0.7]]
                    ];

                    dotPositions[die].forEach(([dx, dy]) => {
                        const dot = two.makeCircle(dieXpos + dx * diceSize / 3, diceYpos + dy * diceSize / 3, diceSize / 12);
                        dot.fill = "black";
                    });
                }
            });
        }

        function drawScores() {
            const boardOrigXpos = width / 2;
            const boardOrigYpos = height / 2;
            const boardWidth = boardCfg.widthFactor * width;
            const boardCheckerSize = (11 / 13) * (boardCfg.widthFactor * width) / 11;

            const score1 = get(positionStore).score[0];
            const score2 = get(positionStore).score[1];         

            const scoreText1 = score1 === 1 ? "crawford" : score1 === 0 ? "post" : score1 === -1 ? "unlimited" : `${score1} away`;
            const scoreText2 = score2 === 1 ? "crawford" : score2 === 0 ? "post" : score2 === -1 ? "unlimited" : `${score2} away`;

            const score1Xpos = boardOrigXpos + boardWidth / 2 + 1.2 * boardCheckerSize;
            const score1Ypos = boardOrigYpos + boardHeight / 2 + 0.2 * boardCheckerSize; // Move closer to the middle

            const score2Xpos = boardOrigXpos + boardWidth / 2 + 1.2 * boardCheckerSize;
            const score2Ypos = boardOrigYpos - boardHeight / 2 - 0.2 * boardCheckerSize; // Move closer to the middle

            // Add visible red rectangles behind the score text
            const redRectangle1 = two.makeRectangle(score1Xpos, score1Ypos, 1.5 * boardCheckerSize, 0.5 * boardCheckerSize);
            redRectangle1.fill = "transparent";
            redRectangle1.stroke = "red"; // Make border visible
            redRectangle1.linewidth = 0;

            const redRectangle2 = two.makeRectangle(score2Xpos, score2Ypos, 1.5 * boardCheckerSize, 0.5 * boardCheckerSize);
            redRectangle2.fill = "transparent";
            redRectangle2.stroke = "red"; // Make border visible
            redRectangle2.linewidth = 0;

            // Add score text
            const scoreText1Element = two.makeText(scoreText1, score1Xpos, score1Ypos - (score1 === 0 ? 10 : 0));
            scoreText1Element.size = 20;
            scoreText1Element.alignment = "center";
            scoreText1Element.baseline = "middle";
            scoreText1Element.weight = "bold";
            if (score1 === 0) {
                const scoreText1Element2 = two.makeText("crawford", score1Xpos, score1Ypos + 10);
                scoreText1Element2.size = 20;
                scoreText1Element2.alignment = "center";
                scoreText1Element2.baseline = "middle";
                scoreText1Element2.weight = "bold";
            }

            const scoreText2Element = two.makeText(scoreText2, score2Xpos, score2Ypos - (score2 === 0 ? 10 : 0));
            scoreText2Element.size = 20;
            scoreText2Element.alignment = "center";
            scoreText2Element.baseline = "middle";
            scoreText2Element.weight = "bold";
            if (score2 === 0) {
                const scoreText2Element2 = two.makeText("crawford", score2Xpos, score2Ypos + 10);
                scoreText2Element2.size = 20;
                scoreText2Element2.alignment = "center";
                scoreText2Element2.baseline = "middle";
                scoreText2Element2.weight = "bold";
            }

            // Add transparent green rectangles on top of the score text
            const greenRectangle1 = two.makeRectangle(score1Xpos, score1Ypos, 1.5 * boardCheckerSize, 0.5 * boardCheckerSize);
            greenRectangle1.fill = "transparent";
            greenRectangle1.stroke = "transparent"; // Make border invisible
            greenRectangle1.linewidth = 2;

            const greenRectangle2 = two.makeRectangle(score2Xpos, score2Ypos, 1.5 * boardCheckerSize, 0.5 * boardCheckerSize);
            greenRectangle2.fill = "transparent";
            greenRectangle2.stroke = "transparent"; // Make border invisible
            greenRectangle2.linewidth = 2;
        }

        const labels = createLabels();

        const quadrant4 = createQuadrant(
            boardOrigXpos + 0.5 * boardCheckerSize,
            boardOrigYpos - boardTriangleHeight - 0.5 * boardCheckerSize,
            false,
        );

        const quadrant3 = createQuadrant(
            boardOrigXpos - 0.5 * boardWidth,
            boardOrigYpos - boardTriangleHeight - 0.5 * boardCheckerSize,
            false,
        );

        const quadrant2 = createQuadrant(
            boardOrigXpos - 0.5 * boardWidth,
            boardOrigYpos + 0.5 * boardCheckerSize,
            true,
        );

        const quadrant1 = createQuadrant(
            boardOrigXpos + 0.5 * boardCheckerSize,
            boardOrigYpos + 0.5 * boardCheckerSize,
            true,
        );

        // draw bar first to ensure checkers on the bar are drawn above it
        const bar = two.makeRectangle(
            boardOrigXpos,
            boardOrigYpos,
            boardCheckerSize,
            boardHeight,
        );
        bar.fill = boardCfg.fill;
        bar.stroke = boardCfg.stroke;
        bar.linewidth = 3.5; // Changed linewidth to 3.5

        drawDoublingCube();
        drawCheckers();
        drawBearoff();        
        drawPipCounts();
        drawDice();
        drawScores();
        
        // Draw arrows for checker movements when showing initial position
        drawMoveArrows(boardOrigXpos, boardOrigYpos, boardWidth, boardHeight, boardCheckerSize);

        // draw board outline on top to ensure consistent linewidth
        const board = two.makeRectangle(
            boardOrigXpos,
            boardOrigYpos,
            boardWidth,
            boardHeight,
        );
        board.fill = "transparent"; // No fill to avoid covering other elements
        board.stroke = boardCfg.stroke;
        board.linewidth = 3.5;
        
        two.update();
    }
</script>

<div class="canvas-container">
    <div id="backgammon-board" class="full-size-board"></div>
</div>

<style>
    body,
    html {
        height: 100%;
        width: 100%;
        margin: 0;
        display: flex;
        justify-content: center;
        align-items: center;
        flex-direction: column;
    }

    .canvas-container {
        width: 100%;
        height: 100%;
        display: flex;
        justify-content: center;
        align-items: center;
        margin: 0;
        padding: 0;
        box-sizing: border-box;
        flex: 1;
    }

    #backgammon-board {
        width: 100%;
        height: 100%;
        box-sizing: border-box;
        padding: 0;
        margin: 0;
        user-select: none; /* Prevent text or element selection */
    }
</style>
