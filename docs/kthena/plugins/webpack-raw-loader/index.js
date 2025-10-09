export default function (context, options) {
  return {
    name: 'webpack-raw-loader',
    configureWebpack(config, isServer, utils) {
      return {
        module: {
          rules: [
            {
              test: /\.ya?ml$/,
              use: 'raw-loader',
            },
          ],
        },
      };
    },
  };
}
