import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */
const sidebars: SidebarsConfig = {
  // Kthena documentation sidebar
  tutorialSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      items: ['getting-started/quick-start', 'getting-started/installation'],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/architecture',
        'architecture/infer-controller',
        'architecture/infer-router',
        'architecture/autoscaler',
        'architecture/model-controller',
      ],
    },
    {
      type: 'category',
      label: 'General',
      items: ['general/prometheus', 'general/cert-manager', 'general/faq'],
    },
    {
      type: 'category',
      label: 'User Guide',
      items: [
        'user-guide/router-routing',
        'user-guide/prefill-decode-disaggregation',
        'user-guide/multi-node-inference',
        'user-guide/config-router',
        'user-guide/autoscaler',
        'user-guide/rate-limit',
        'user-guide/runtime',
      ],
    },
    {
      type: 'category',
      label: 'Developer Guide',
      items: [
        'developer-guide/release',
        'developer-guide/modelinfer-rolling-update',
        'developer-guide/ci',
        'developer-guide/modelinfer-scaling',
      ],
    },
    {
      type: 'category',
      label: 'Timeline',
      items: ['timeline/roadmap', 'timeline/releases'],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        {
          type: 'doc',
          id: 'reference/router-access-log-fields',
          label: 'Router Access Log Fields',
        },
        {
          type: 'category',
          label: 'CRD Reference',
          items: [
            {
              type: 'doc',
              id: 'reference/crd/networking.volcano.sh',
              label: 'Networking',
            },
            {
              type: 'doc',
              id: 'reference/crd/registry.volcano.sh',
              label: 'Registry',
            },
            {
              type: 'doc',
              id: 'reference/crd/workload.volcano.sh',
              label: 'Workload',
            },
          ],
        },
        {
          type: 'category',
          label: 'Minfer CLI',
          items: [
            { type: 'doc', id: 'reference/cli/minfer', label: 'Minfer' },
            { type: 'doc', id: 'reference/cli/minfer_create', label: 'Create' },
            { type: 'doc', id: 'reference/cli/minfer_get', label: 'Get' },
            {
              type: 'doc',
              id: 'reference/cli/minfer_describe',
              label: 'Describe',
            },
          ],
        },
      ],
    },
  ],
};

export default sidebars;
