## Contributing to Documentation

This section provides guidelines for developers who want to contribute to the MatrixInfer documentation.

### Adding New Documentation

1. **Create a new markdown file** in the appropriate directory under `docs/`
2. **Update the sidebar** by editing `sidebars.ts` to include your new page in the navigation
3. **Follow the naming convention**: Use lowercase with hyphens (e.g., `my-new-feature.md`)

### Writing Guidelines

- Use clear, concise language
- Include code examples where applicable
- Add proper headings hierarchy (H1 for page title, H2 for main sections, etc.)
- Use markdown formatting consistently
- Include links to related documentation when relevant

### Testing Your Changes

1. **Start the development server**:
   ```bash
   npm run start
   ```
2. **Preview your changes** in the browser at `http://localhost:3000`
3. **Check for broken links** and ensure navigation works correctly
4. **Build the site** to verify no build errors:
   ```bash
   npm run build
   ```

### Sidebar Configuration

To add your new documentation to the sidebar navigation, edit `sidebars.ts`:

```typescript
const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Your Category',
      items: [
        'your-category/your-new-page',
      ],
    },
  ],
};
```

### Contribution Workflow

1. Create a new branch for your documentation changes
2. Add or modify documentation files
3. Update `sidebars.ts` if adding new pages
4. Test locally using `npm run start`
5. Build the site to ensure no errors: `npm run build`
6. Submit a pull request with a clear description of your changes


## Deployment

Using SSH:

```bash
USE_SSH=true npm run deploy
```

Not using SSH:

```bash
GIT_USER=<Your GitHub username> npm run deploy
```

If you are using GitHub pages for hosting, this command is a convenient way to build the website and push to the `gh-pages` branch.