# memorygame

Memorygame is a React App designed to practice my daughter's ability to memorize
sequences of colors. Similar to the old "Simon Says" toys, this game has a four
colored buttons and plays sounds while the game is running.

When the webpage is first loaded, it will show the colored buttons (Red, Green, Yellow,
and Blue) in the background and have a large "Start" button overlayed on top and in the
center. When the "Start" button is clicked the game will have a "3", "2", "1" count down
and then it will begin.  While the game is playing there will be some fun background
music playing. The App will have Volume and Mute buttons in the top right corner. The
game will switch between "Recording" and "Playback" phases. During "Recording" phase, the
computer player will randomly pick a new colored button that will be appended to the
end of the currently memorized sequence. The computer player will blink each color the
same memorized sequence one after the other. Once the last color is blinked, which is the
newest color, then the "Recording" phase ends and the "Playback" phase begins. During
the "Playback" phase, the player will tap each colored button in the order that they
remember. After each button press the game will play the corresponding color sound. If
one color button does not match the memorized sequence from the "Recording" phase then
there will be "uh oh" sound and the game will end.

## Running Instructions

If not done yet, `sudo apt-get install npm` or install nvm to manage npm

`npm run dev -- --host` or similar. Default will open on port `:5173`

If running on a separate domain, then update `vite.config.js` to include the following:
```
server: {
  allowedHosts: ['{your domain here}']
}
```
