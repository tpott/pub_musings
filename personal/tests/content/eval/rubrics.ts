export interface Dimension {
  name: string;
  description: string;
}

export interface Rubric {
  name: string;
  slug: string;
  dimensions: Dimension[];
}

export const rubrics: Rubric[] = [
  {
    name: 'Comprehension',
    slug: 'comprehension',
    dimensions: [
      { name: 'clarity', description: 'How clear and understandable is the writing? Can the reader easily follow the main points?' },
      { name: 'structure', description: 'How well-organized is the post? Does it have a logical flow from introduction to conclusion?' },
      { name: 'accessibility', description: 'How accessible is the content to the target audience? Are technical terms explained when needed?' },
    ],
  },
  {
    name: 'Engagement',
    slug: 'engagement',
    dimensions: [
      { name: 'hook', description: 'Does the opening draw the reader in and make them want to continue?' },
      { name: 'voice', description: 'Does the author have a distinctive, authentic voice? Does the writing feel personal rather than generic?' },
      { name: 'payoff', description: 'Does the post deliver on its premise? Does the reader feel their time was well spent?' },
    ],
  },
  {
    name: 'Rigor',
    slug: 'rigor',
    dimensions: [
      { name: 'accuracy', description: 'Are technical claims correct and well-supported? Are sources cited when making specific claims?' },
      { name: 'depth', description: 'Does the post go beyond surface-level treatment? Does it show genuine understanding of the topic?' },
      { name: 'nuance', description: 'Does the author acknowledge trade-offs, limitations, or alternative perspectives?' },
    ],
  },
  {
    name: 'Perception',
    slug: 'perception',
    dimensions: [
      { name: 'credibility', description: 'Does the post make the author seem knowledgeable and trustworthy?' },
      { name: 'professionalism', description: 'Is the writing polished? Free of obvious errors, consistent in tone?' },
      { name: 'memorability', description: 'Would the reader remember this post or recommend it to others? Does it have a lasting impression?' },
    ],
  },
];
