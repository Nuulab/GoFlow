const { createPreset } = require('fumadocs-ui/tailwind-plugin');

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './src/components/**/*.{ts,tsx}',
    './src/app/**/*.{ts,tsx}',
    './content/**/*.{md,mdx}',
    './node_modules/fumadocs-ui/dist/**/*.js',
  ],
  presets: [createPreset()],
};
