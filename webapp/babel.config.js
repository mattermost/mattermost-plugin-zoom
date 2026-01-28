// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// eslint-disable-next-line no-process-env
const isTest = process.env.NODE_ENV === 'test';

const config = {
    presets: [
        ['@babel/preset-env', {
            targets: {
                chrome: 66,
                firefox: 60,
                edge: 42,
                safari: 12,
            },
            modules: isTest ? 'commonjs' : false,
            corejs: 3,
            debug: false,
            useBuiltIns: 'usage',
            shippedProposals: true,
        }],
        ['@babel/preset-react', {
            runtime: 'automatic',
        }],
        ['@babel/typescript', {
            allExtensions: true,
            isTSX: true,
        }],
        ['@emotion/babel-preset-css-prop'],
    ],
    plugins: [
        '@babel/plugin-transform-class-properties',
        '@babel/plugin-syntax-dynamic-import',
        '@babel/plugin-transform-object-rest-spread',
        '@babel/plugin-transform-optional-chaining',
        'babel-plugin-typescript-to-proptypes',
    ],
};

module.exports = config;
