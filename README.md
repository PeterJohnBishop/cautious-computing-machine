# cautious-computing-machine


# TUI Requirements 
- Toggle Send / Receive TUI
- Sender: Text Input for file path 
- Sender: Text Input for defining your unique identifier 
- Receiver: Text Input for defining who you'll receive from (sender unique identifier)
- Receiver: Text Input for defining save path for file

# Sender Flow
- Sender inputs the path to the file they want to send. 
- Sender inputs or generates a unique identifier.
- Sender connects to the signaling server with a unique identifier.
- Sender waits for the receiver to connect. 
- Sender initiates a webRTC handshake.
- Sender chunks the file and sends the manifest. 
- Sender sends the chunks.

# Receiver Flow
- Inputs the path to the directory where the file should be saved. 
- Inputs the unique identifier of the sender. 
- Connects to the signaling server and finds or waits for the matching sender. 
- Receives the manifest and chunked data. 
- Receiver sends a confirmation message and closes the webRTC data channel. 



