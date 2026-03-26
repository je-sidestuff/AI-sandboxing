const express = require('express');
const http = require('http');
const path = require('path');
const fs = require('fs');
const { Server } = require('socket.io');
const { TranscribeStreamingClient, StartStreamTranscriptionCommand } = require("@aws-sdk/client-transcribe-streaming");
const jwt = require('jsonwebtoken');
const { v4: uuidv4 } = require('uuid');

const app = express();
const server = http.createServer(app);
const io = new Server(server);

// JWT configuration
const JWT_SECRET = 'your-secret-key-change-in-production'; // TODO: Move to environment variables

// Heuristic output configuration
const HEURISTIC_BASE_DIR = process.env.HEURISTIC_DIR || '/workspaces/slopspaces/heuristic';
const HEURISTIC_PENDING_DIR = path.join(HEURISTIC_BASE_DIR, 'pending');

// Prelude and postlude text for heuristic intake
const HEURISTIC_PRELUDE = 'In je-sidestuff/AI-sandboxing - ';
const HEURISTIC_POSTLUDE = ' Use repo-isolation to avoid interfering with the target repo.';

// Generate a sample JWT on startup for testing
const sampleToken = jwt.sign(
    {
        userId: 'test-user',
        iat: Math.floor(Date.now() / 1000),
        exp: Math.floor(Date.now() / 1000) + (60 * 60 * 24) // 24 hours
    },
    JWT_SECRET
);

console.log('\n=== JWT Authentication Sample Token ===');
console.log('Use this token for testing:');
console.log(sampleToken);
console.log('=======================================\n');

app.use(express.static(path.join(__dirname)));

app.get('/', (req, res) => {
    res.sendFile(path.join(__dirname, 'index.html'));
});

// Endpoint to get a test JWT token
app.get('/api/token', (req, res) => {
    const token = jwt.sign(
        {
            userId: 'test-user',
            iat: Math.floor(Date.now() / 1000),
            exp: Math.floor(Date.now() / 1000) + (60 * 60 * 24) // 24 hours
        },
        JWT_SECRET
    );
    res.json({ token });
});

const transcribeClient = new TranscribeStreamingClient({
    region: "us-west-2", // Ensure this matches your AWS region
});

// Function to save transcription to HEURISTIC.md for intake
function saveToHeuristic(transcriptText, callback) {
    const heuristicId = uuidv4().slice(0, 8);
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    const folderName = `${timestamp}_scribe_${heuristicId}`;
    const folderPath = path.join(HEURISTIC_PENDING_DIR, folderName);
    const heuristicFile = path.join(folderPath, 'HEURISTIC.md');

    // Format the content with prelude and postlude
    const heuristicContent = `${HEURISTIC_PRELUDE}${transcriptText}${HEURISTIC_POSTLUDE}`;

    // Ensure the pending directory exists
    fs.mkdir(folderPath, { recursive: true }, (err) => {
        if (err) {
            console.error('Error creating heuristic folder:', err);
            callback(err);
            return;
        }

        // Write the HEURISTIC.md file
        fs.writeFile(heuristicFile, heuristicContent, (err) => {
            if (err) {
                console.error('Error writing HEURISTIC.md:', err);
                callback(err);
            } else {
                console.log(`Transcription saved to ${heuristicFile}`);
                callback(null, heuristicFile);
            }
        });
    });
}

// Function to save direct text input to HEURISTIC.md (no prelude/postlude)
function saveDirectTextToHeuristic(text, callback) {
    const heuristicId = uuidv4().slice(0, 8);
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    const folderName = `${timestamp}_scribe_${heuristicId}`;
    const folderPath = path.join(HEURISTIC_PENDING_DIR, folderName);
    const heuristicFile = path.join(folderPath, 'HEURISTIC.md');

    // Use the text directly without prelude/postlude
    const heuristicContent = text;

    fs.mkdir(folderPath, { recursive: true }, (err) => {
        if (err) {
            console.error('Error creating heuristic folder:', err);
            callback(err);
            return;
        }

        fs.writeFile(heuristicFile, heuristicContent, (err) => {
            if (err) {
                console.error('Error writing HEURISTIC.md:', err);
                callback(err);
            } else {
                console.log(`Direct text saved to ${heuristicFile}`);
                callback(null, heuristicFile);
            }
        });
    });
}

// JWT verification middleware for Socket.IO
io.use((socket, next) => {
    const token = socket.handshake.auth.token;

    if (!token) {
        console.log('Connection rejected: No token provided');
        return next(new Error('Authentication error: No token provided'));
    }

    jwt.verify(token, JWT_SECRET, (err, decoded) => {
        if (err) {
            console.log('Connection rejected: Invalid token', err.message);
            return next(new Error('Authentication error: Invalid token'));
        }

        // Attach decoded token data to socket for later use
        socket.userId = decoded.userId;
        console.log('User authenticated:', decoded.userId);
        next();
    });
});

io.on('connection', (socket) => {
    console.log('A user connected with userId:', socket.userId);

    let audioStream;
    let lastTranscript = '';
    let isTranscribing = false;
    let sessionTranscript = ''; // Accumulate full session transcript

    socket.on('startTranscription', async () => {
        console.log('Starting transcription');
        isTranscribing = true;
        sessionTranscript = ''; // Reset session transcript
        let buffer = Buffer.from('');

        audioStream = async function* () {
            while (isTranscribing) {
                const chunk = await new Promise(resolve => socket.once('audioData', resolve));
                if (chunk === null) break;
                buffer = Buffer.concat([buffer, Buffer.from(chunk)]);
                console.log('Received audio chunk, buffer size:', buffer.length);

                while (buffer.length >= 1024) {
                    yield { AudioEvent: { AudioChunk: buffer.slice(0, 1024) } };
                    buffer = buffer.slice(1024);
                }
            }
        };

        const command = new StartStreamTranscriptionCommand({
            LanguageCode: "en-US",
            MediaSampleRateHertz: 44100,
            MediaEncoding: "pcm",
            AudioStream: audioStream()
        });

        try {
            console.log('Sending command to AWS Transcribe');
            const response = await transcribeClient.send(command);
            console.log('Received response from AWS Transcribe');

            for await (const event of response.TranscriptResultStream) {
                if (!isTranscribing) break;
                if (event.TranscriptEvent) {
                    console.log('Received TranscriptEvent:', JSON.stringify(event.TranscriptEvent));
                    const results = event.TranscriptEvent.Transcript.Results;
                    if (results.length > 0 && results[0].Alternatives.length > 0) {
                        const transcript = results[0].Alternatives[0].Transcript;
                        const isFinal = !results[0].IsPartial;

                        if (isFinal) {
                            console.log('Emitting final transcription:', transcript);
                            socket.emit('transcription', { text: transcript, isFinal: true });
                            lastTranscript = transcript;

                            // Accumulate to session transcript
                            sessionTranscript += transcript + ' ';
                        } else {
                            const newPart = transcript.substring(lastTranscript.length);
                            if (newPart.trim() !== '') {
                                console.log('Emitting partial transcription:', newPart);
                                socket.emit('transcription', { text: newPart, isFinal: false });
                            }
                        }
                    }
                }
            }
        } catch (error) {
            console.error("Transcription error:", error);
            socket.emit('error', 'Transcription error occurred: ' + error.message);
        }
    });

    socket.on('audioData', (data) => {
        if (isTranscribing) {
            console.log('Received audioData event, data size:', data.byteLength);
            socket.emit('audioData', data);
        }
    });

    socket.on('stopTranscription', () => {
        console.log('Stopping transcription');
        isTranscribing = false;
        audioStream = null;
        lastTranscript = '';
    });

    // Save the accumulated transcript to HEURISTIC.md
    socket.on('saveTranscription', () => {
        console.log('Saving transcription to HEURISTIC.md');
        const trimmedTranscript = sessionTranscript.trim();

        if (!trimmedTranscript) {
            socket.emit('saveResult', { success: false, error: 'No transcription to save' });
            return;
        }

        saveToHeuristic(trimmedTranscript, (err, filePath) => {
            if (err) {
                socket.emit('saveResult', { success: false, error: err.message });
            } else {
                socket.emit('saveResult', { success: true, filePath: filePath });
                // Clear the session transcript after successful save
                sessionTranscript = '';
            }
        });
    });

    // Save direct text input to HEURISTIC.md (no prelude/postlude)
    socket.on('saveTextInput', (text) => {
        console.log('Saving direct text input to HEURISTIC.md');
        const trimmedText = (text || '').trim();

        if (!trimmedText) {
            socket.emit('textSaveResult', { success: false, error: 'No text to save' });
            return;
        }

        saveDirectTextToHeuristic(trimmedText, (err, filePath) => {
            if (err) {
                socket.emit('textSaveResult', { success: false, error: err.message });
            } else {
                socket.emit('textSaveResult', { success: true, filePath: filePath });
            }
        });
    });

    socket.on('disconnect', () => {
        console.log('User disconnected');
        isTranscribing = false;
        audioStream = null;
    });
});

const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
    console.log(`Server is running on http://localhost:${PORT}`);
    console.log(`Heuristic output: ${HEURISTIC_PENDING_DIR}`);
});
