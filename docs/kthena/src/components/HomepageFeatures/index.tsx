import type { ReactNode } from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  Svg: React.ComponentType<React.ComponentProps<'svg'>>;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Kubernetes Native',
    Svg: require('@site/static/img/homepage/kthena-feature-1.svg').default,
    description: (
      <>
        Kthena is designed from the ground up to be Kubernetes-native, providing
        seamless integration with your existing K8s infrastructure.
      </>
    ),
  },
  {
    title: 'Intelligent Scaling',
    Svg: require('@site/static/img/homepage/kthena-feature-2.svg').default,
    description: (
      <>
        Focus on your AI models while Kthena handles intelligent auto-scaling
        and routing. Deploy models with confidence knowing they&apos;ll scale
        efficiently.
      </>
    ),
  },
  {
    title: 'Multi-Model Serving',
    Svg: require('@site/static/img/homepage/kthena-feature-3.svg').default,
    description: (
      <>
        Serve multiple AI models simultaneously with advanced model management
        capabilities. Kthena supports diverse model formats and frameworks.
      </>
    ),
  },
];

function Feature({ title, Svg, description }: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center">
        <Svg className={styles.featureSvg} role="img" />
      </div>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
